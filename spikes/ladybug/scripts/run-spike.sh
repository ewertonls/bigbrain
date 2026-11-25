#!/bin/sh
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
artifacts_root="$project_dir/artifacts"
dist_root="$project_dir/dist"
runtime_image=bigbrain-ladybug-spike:v0.17.0
source_image=bigbrain-ladybug-migration:v0.13.1
offline_base=debian@sha256:e67eeead736ff7bbe258ef088bff683635c561b1fcfefad0806c1df4ab49ce0e

mkdir -p "$artifacts_root" "$dist_root"
run_dir=$(mktemp -d "$artifacts_root/run.XXXXXX")
dist_dir=$(mktemp -d "$dist_root/run.XXXXXX")

docker build --platform linux/arm64 --target runtime --tag "$runtime_image" "$project_dir"
docker build --platform linux/arm64 --target migration-source --tag "$source_image" "$project_dir"
docker build --platform linux/arm64 --target package --output "type=local,dest=$dist_dir/package" "$project_dir"
docker pull --platform linux/arm64 "$offline_base"

environment_dir="$run_dir/environment"
mkdir -p "$environment_dir"
docker run --rm --platform linux/arm64 --volume "$environment_dir:/work" "$runtime_image" schema --db /work/environment.lbdb > "$run_dir/environment-schema.json"
docker run --rm --platform linux/arm64 --volume "$environment_dir:/work" "$runtime_image" seed --db /work/environment.lbdb > "$run_dir/environment-seed.json"
docker run --rm --platform linux/arm64 --volume "$environment_dir:/work" "$runtime_image" verify --db /work/environment.lbdb > "$run_dir/environment-verify.json"

transactions_dir="$run_dir/transactions"
mkdir -p "$transactions_dir"
docker run --rm --platform linux/arm64 --volume "$transactions_dir:/work" "$runtime_image" schema --db /work/transactions.lbdb > /dev/null
docker run --rm --platform linux/arm64 --volume "$transactions_dir:/work" "$runtime_image" seed --db /work/transactions.lbdb > /dev/null
docker run --rm --platform linux/arm64 --volume "$transactions_dir:/work" "$runtime_image" transactions --db /work/transactions.lbdb --samples 100 --writes 25 > "$run_dir/transactions.json"

unsafe_dir="$run_dir/unsafe-concurrency"
mkdir -p "$unsafe_dir"
docker run --rm --platform linux/arm64 --volume "$unsafe_dir:/work" "$runtime_image" schema --db /work/unsafe.lbdb > /dev/null
docker run --rm --platform linux/arm64 --volume "$unsafe_dir:/work" "$runtime_image" seed --db /work/unsafe.lbdb > /dev/null
set +e
docker run --rm --platform linux/arm64 --volume "$unsafe_dir:/work" "$runtime_image" transactions --db /work/unsafe.lbdb --samples 5 --writes 1 --serialize-writes=false > "$run_dir/unsafe-concurrency.txt" 2>&1
unsafe_exit=$?
set -e
printf '%s\n' "$unsafe_exit" > "$run_dir/unsafe-concurrency-exit.txt"

crash_dir="$run_dir/crash"
mkdir -p "$crash_dir"
docker run --rm --platform linux/arm64 --volume "$crash_dir:/work" "$runtime_image" schema --db /work/crash.lbdb > /dev/null
docker run --rm --platform linux/arm64 --volume "$crash_dir:/work" "$runtime_image" seed --db /work/crash.lbdb > /dev/null

set +e
docker run --rm --platform linux/arm64 --volume "$crash_dir:/work" "$runtime_image" crash-write --db /work/crash.lbdb --phase before-commit --key crash-before --id CRASH-BEFORE > "$run_dir/crash-before-stdout.txt" 2> "$run_dir/crash-before-stderr.txt"
before_exit=$?
set -e
test "$before_exit" -eq 86
docker run --rm --platform linux/arm64 --volume "$crash_dir:/work" "$runtime_image" inspect-write --db /work/crash.lbdb --key crash-before --id CRASH-BEFORE > "$run_dir/crash-before-inspect.json"
docker run --rm --platform linux/arm64 --volume "$crash_dir:/work" "$runtime_image" retry-write --db /work/crash.lbdb --key crash-before --id CRASH-BEFORE > "$run_dir/crash-before-retry.json"

set +e
docker run --rm --platform linux/arm64 --volume "$crash_dir:/work" "$runtime_image" crash-write --db /work/crash.lbdb --phase after-commit --key crash-after --id CRASH-AFTER > "$run_dir/crash-after-stdout.txt" 2> "$run_dir/crash-after-stderr.txt"
after_exit=$?
set -e
test "$after_exit" -eq 86
docker run --rm --platform linux/arm64 --volume "$crash_dir:/work" "$runtime_image" inspect-write --db /work/crash.lbdb --key crash-after --id CRASH-AFTER > "$run_dir/crash-after-inspect.json"
docker run --rm --platform linux/arm64 --volume "$crash_dir:/work" "$runtime_image" retry-write --db /work/crash.lbdb --key crash-after --id CRASH-AFTER > "$run_dir/crash-after-retry.json"

migration_dir="$run_dir/migration"
mkdir -p "$migration_dir"
docker run --rm --platform linux/arm64 \
    --volume "$migration_dir:/work" \
    --volume "$project_dir/migration:/input:ro" \
    "$source_image" /work/source.lbdb --init /input/v0.13.1-seed.cypher --no_progress_bar --no_stats \
    > "$run_dir/migration-source.txt" 2>&1
docker run --rm --platform linux/arm64 --volume "$migration_dir:/work" "$runtime_image" import --db /work/target.lbdb --export /work/export-v0.13.1 > "$run_dir/migration-import.json"
docker run --rm --platform linux/arm64 --volume "$migration_dir:/work" "$runtime_image" snapshot --db /work/target.lbdb > "$run_dir/migration-snapshot.json"
docker run --rm --platform linux/arm64 --volume "$migration_dir:/work" "$runtime_image" verify --db /work/target.lbdb > "$run_dir/migration-verify.json"
docker run --rm --platform linux/arm64 --volume "$migration_dir:/work" "$runtime_image" retry-write --db /work/target.lbdb --key seed-d1 --id D1 > "$run_dir/migration-replay.json"

package_path="$dist_dir/bigbrain-ladybug-spike-linux-arm64.tar.gz"
COPYFILE_DISABLE=1 tar --no-xattrs -czf "$package_path" -C "$dist_dir/package" bigbrain
if command -v shasum > /dev/null 2>&1; then
    shasum -a 256 "$package_path" > "$run_dir/package.sha256"
else
    sha256sum "$package_path" > "$run_dir/package.sha256"
fi
du -h "$package_path" > "$run_dir/package-size.txt"
find "$dist_dir/package" -maxdepth 3 \( -type f -o -type l \) | sort > "$run_dir/package-files.txt"

docker image inspect "$offline_base" > "$run_dir/offline-base-image.json"
offline_dir="$run_dir/offline"
mkdir -p "$offline_dir"
docker run --rm --platform linux/arm64 --network none \
    --volume "$package_path:/package.tar.gz:ro" \
    --volume "$project_dir/scripts:/scripts:ro" \
    --volume "$offline_dir:/work" \
    "$offline_base" sh /scripts/offline-test.sh > "$run_dir/offline.json"

docker image inspect "$runtime_image" --format '{{.Id}} {{.Architecture}} {{.Os}}' > "$run_dir/runtime-image.txt"
docker run --rm --platform linux/arm64 --entrypoint sh "$runtime_image" -c 'ldd /opt/bigbrain/bin/ladybug-spike; ldd /opt/bigbrain/lib/liblbug.so.0.17.0' > "$run_dir/runtime-dependencies.txt"

printf 'Evidence: %s\nPackage: %s\n' "$run_dir" "$package_path"

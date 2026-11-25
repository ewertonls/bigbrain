#!/bin/sh
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
run_dir=$(mktemp -d "${TMPDIR:-/tmp}/bigbrain-foundation.XXXXXX")
archive="$run_dir/bigbrain-linux-arm64.tar.gz"
image=bigbrain-foundation-acceptance:v1

printf 'Evidence directory: %s\n' "$run_dir"
docker build --platform linux/arm64 --target test "$project_dir"
docker build --platform linux/arm64 --target package --output "type=local,dest=$run_dir/package" "$project_dir"
COPYFILE_DISABLE=1 tar --no-xattrs -czf "$archive" -C "$run_dir/package" bigbrain
docker build --platform linux/arm64 --target acceptance --tag "$image" "$project_dir"
docker run --rm --platform linux/arm64 --network none \
    --volume "$archive:/package.tar.gz:ro" --entrypoint sh "$image" -ec '
        test "$(uname -m)" = aarch64
        test -z "$(command -v cc || true)"
        test -z "$(command -v go || true)"
        test "$(wc -l < /proc/net/route)" -eq 1
        mkdir /tmp/install
        tar -xzf /package.tar.gz -C /tmp/install
        test -s /tmp/install/bigbrain/licenses/ladybug.txt
        test -s /tmp/install/bigbrain/licenses/go-ladybug.txt
        ldd /tmp/install/bigbrain/bin/bigbrain >&2
        exec /opt/acceptance/bin/acceptance /tmp/install/bigbrain/bin/bigbrain /opt/acceptance/bin/bigbrain-faults
    ' > "$run_dir/acceptance.json"
cat "$run_dir/acceptance.json"
printf 'Package: %s\n' "$archive"

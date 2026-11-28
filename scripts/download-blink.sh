#!/bin/sh
set -eu
revision=de97a396996d93b039aa48463ab34f5459657529
url=https://huggingface.co/minishlab/blink-multilingual-64M-int8/resolve/$revision
manifest=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)/blink.sha256
dest=${1:-build/models/blink-multilingual-64M-int8}
verify() (
    cd -- "$1"
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum -c "$manifest"
    else
        shasum -a 256 -c "$manifest"
    fi
)
if [ -e "$dest" ]; then
    verify "$dest" || { echo "Choose a new directory to install a verified Blink bundle." >&2; exit 1; }
    exit 0
fi
mkdir -p -- "$(dirname -- "$dest")"
stage=$(mktemp -d "$dest.download.XXXXXX")
trap 'rm -rf -- "$stage"' EXIT
trap 'exit 1' HUP INT TERM
for name in config.json tokenizer.json model.safetensors; do
    curl --fail --location --silent --show-error --retry 3 "$url/$name" -o "$stage/$name"
done
verify "$stage"
mv -- "$stage" "$dest"

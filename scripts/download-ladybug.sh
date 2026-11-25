#!/bin/sh
set -eu

version=0.17.0
os=$(uname -s)
arch=$(uname -m)
case "$os/$arch" in
    Darwin/arm64)
        platform=osx-arm64
        sha256=f2acc15c9a90874aa25cebd49921080e44774525b061ec7189fae18d1878d018 ;;
    Darwin/x86_64)
        platform=osx-x86_64
        sha256=8ab59121a1aadeaeeca1f855cfc4d3a596620a737461976e6e084f02c6cf5b0a ;;
    Linux/aarch64|Linux/arm64)
        platform=linux-aarch64
        sha256=a486000f7305cb601e3074eb06026e9eddcf3d7517df7bbc913d2d36e3108121 ;;
    Linux/x86_64)
        platform=linux-x86_64
        sha256=f546b37616347533c5d08a38fd1c9f51bfe5e2c97c56d68582e07e675f546d4d ;;
    *)
        echo "Unsupported LadybugDB platform: $os/$arch (expected macOS or Linux on ARM64 or x86-64)." >&2
        exit 1 ;;
esac

case "$os" in
    Darwin)
        library=liblbug.$version.dylib
        soname=liblbug.0.dylib
        linker_name=liblbug.dylib ;;
    Linux)
        if ! getconf GNU_LIBC_VERSION >/dev/null 2>&1; then
            echo "The LadybugDB Linux release requires glibc (checked with getconf GNU_LIBC_VERSION)." >&2
            exit 1
        fi
        library=liblbug.so.$version
        soname=liblbug.so.0
        linker_name=liblbug.so ;;
esac

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
mkdir -p "$root/lib-ladybug"
cd "$root/lib-ladybug"

# Reuse the installation only for the same release and host architecture.
if [ "$(cat .installed 2>/dev/null || true)" = "$version $platform" ] &&
   [ -s lbug.h ] && [ -s "$library" ] &&
   [ "$(readlink "$linker_name")" = "$soname" ] &&
   [ "$(readlink "$soname")" = "$library" ]; then
    echo "LadybugDB $version is ready for $platform."
    exit 0
fi

stage=$(mktemp -d .download.XXXXXX)
trap 'rm -rf "$stage"' EXIT
trap 'exit 1' HUP INT TERM
url="https://github.com/LadybugDB/ladybug/releases/download/v$version/liblbug-$platform.tar.gz"
echo "Downloading LadybugDB $version for $platform..."
curl --fail --location --silent --show-error --retry 3 --output "$stage/archive" "$url"
if command -v sha256sum >/dev/null 2>&1; then
    digest=$(sha256sum "$stage/archive")
else
    digest=$(shasum -a 256 "$stage/archive")
fi
if [ "${digest%% *}" != "$sha256" ]; then
    echo "SHA-256 mismatch for $url" >&2
    exit 1
fi
tar -xzf "$stage/archive" -C "$stage" lbug.h "$library"
ln -s "$library" "$stage/$soname"
ln -s "$soname" "$stage/$linker_name"
echo "$version $platform" > "$stage/.installed"
for file in lbug.h "$library" "$soname" "$linker_name" .installed; do
    mv -f "$stage/$file" "$file"
done

#!/usr/bin/env bash
# Run after the workflow has fetched the bundle, generated reference vectors and built the binaries.
set -euo pipefail
repo=$(cd "$(dirname "$0")/../.." && pwd -P)
work=$(cd "$1" && pwd -P)
binary="$work/relocated/bin/bigbrain"
export BIGBRAIN_BLINK_BUNDLE="$work/model" BIGBRAIN_EMBEDDINGS="$work/model" BIGBRAIN_VECTOR_INDEX=
cd "$work"

case "$(uname -sm)" in
  "Linux x86_64")
    python3 - <<'PY'
import errno
import os
import socket

assert os.geteuid() != 0
assert [name for _, name in socket.if_nameindex()] == ["lo"], "expected only loopback"
for family, address in ((socket.AF_INET, "192.0.2.1"), (socket.AF_INET6, "2001:db8::1")):
    try:
        with socket.socket(family, socket.SOCK_DGRAM) as probe:
            probe.connect((address, 9))
    except OSError as exc:
        assert exc.errno in (errno.ENETUNREACH, errno.EHOSTUNREACH, errno.EADDRNOTAVAIL, errno.EAFNOSUPPORT), exc
    else:
        raise RuntimeError(f"reachable external route to {address}")
left, right = socket.socketpair(socket.AF_UNIX)
with left, right:
    left.sendall(b"ipc")
    assert right.recv(3) == b"ipc"
print("Verified Linux network namespace: no external IPv4/IPv6 route, Unix IPC available")
PY
    ;;
  "Darwin arm64")
    unset DYLD_LIBRARY_PATH DYLD_FALLBACK_LIBRARY_PATH
    otool -L "$binary"
    DYLD_PRINT_LIBRARIES=1 "$binary" help > /dev/null 2> "$work/dyld.log"
    grep -F '/relocated/lib/liblbug.0.dylib' "$work/dyld.log"
    echo "Testing native macOS ARM64 with the relocated dylib; networking is available."
    ;;
  *) echo "Unexpected validation host: $(uname -sm)" >&2; exit 1 ;;
esac

# The pinned official runtime produced the expected data during setup.
(cd "$work/results/reference" && "$work/blink.test" -test.v -test.run '^TestReleaseGoldens$')
if [[ -f "$repo/internal/blink/testdata/blink.json" ]]; then
    (cd "$repo/internal/blink" && "$work/blink.test" -test.v -test.run '^TestReleaseGoldens$')
fi
(cd "$repo/internal/store" && "$work/store.test" -test.v \
    -test.run '^Test(BlinkCutoffCalibration|VectorCacheAcrossScopesAndRestarts|VectorRebuildResumesCommittedWork|DamagedVectorEntryAndUnavailableCache)$')
python3 "$repo/tests/memory/cli_blink.py" "$binary" "$work/model"
for fixture in fixture held-out; do
    "$work/acceptance" --memory "$binary" "$repo/tests/memory/$fixture.json" | tee "$work/results/$fixture.json"
done

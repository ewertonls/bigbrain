"""Verify the runner's process-level IP restrictions and named Unix IPC."""
import errno
import os
from pathlib import Path
import socket
import sys
import tempfile


def blocked(family, address, kind, expected):
    try:
        with socket.socket(family, kind) as probe:
            probe.settimeout(2)
            probe.connect((address, 9))
    except OSError as exc:
        if exc.errno not in expected:
            raise
    else:
        raise RuntimeError(f"network access to {address} is available")


if sys.platform == "linux":
    assert os.geteuid() != 0
    assert [name for _, name in socket.if_nameindex()] == ["lo"], "expected only loopback"
    for family, address in ((socket.AF_INET, "192.0.2.1"), (socket.AF_INET6, "2001:db8::1")):
        blocked(family, address, socket.SOCK_DGRAM, (errno.ENETUNREACH, errno.EHOSTUNREACH, errno.EADDRNOTAVAIL, errno.EAFNOSUPPORT))
elif sys.platform == "darwin":
    # An explicit permission error on loopback distinguishes Seatbelt from a
    # missing route, a refused port or an unavailable external service.
    for family, address in ((socket.AF_INET, "127.0.0.1"), (socket.AF_INET6, "::1")):
        for kind in (socket.SOCK_STREAM, socket.SOCK_DGRAM):
            blocked(family, address, kind, (errno.EACCES, errno.EPERM))
else:
    raise RuntimeError(f"unsupported host {sys.platform}")

with tempfile.TemporaryDirectory(dir="/tmp") as tmp:
    address = str(Path(tmp) / "ipc.sock")
    with socket.socket(socket.AF_UNIX) as listener, socket.socket(socket.AF_UNIX) as client:
        listener.bind(address)
        listener.listen()
        client.connect(address)
        peer, _ = listener.accept()
        with peer:
            client.sendall(b"ipc")
            assert peer.recv(3) == b"ipc"
print(f"Verified {sys.platform} IP restriction and named Unix IPC")

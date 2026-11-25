# LadybugDB technical spike

This directory contains the Linux ARM64 experiment for BB-0001. It pins
LadybugDB and `go-ladybug` to `v0.17.0`, uses the official `v0.13.1` CLI as
the migration source, and verifies the Orbit fixture from `docs/SPEC.md`.

The completed decision and measurements are in [REPORT.md](REPORT.md).

## Run

Docker Desktop must be running on an ARM64 host. From this directory, run:

```sh
./scripts/run-spike.sh
```

The script creates a fresh directory under `artifacts/` for JSON and diagnostic
evidence and a fresh directory under `dist/` for the package. Both directories
are ignored by Git. It does not delete or overwrite earlier runs.

The experiment covers:

- opening, persisting, reopening, and traversing the Orbit graph in both
  directions;
- visibility across two connections, manual commit and rollback, a serialized
  writer queue, and measured startup and one-hop query latency;
- process death before commit and after commit but before acknowledgment;
- export on `v0.13.1`, import into an empty `v0.17.0` database, snapshot
  comparison, and idempotent replay;
- installation of the generated package in a network-disabled Debian 12 ARM64
  container with no Go toolchain or C compiler.

## Concurrency constraint

The `v0.17.0` Go/native pair crashes in `lbug_connection_prepare` when two Go
goroutines issue prepared writes simultaneously through separate connections
from one database object. The normal transaction experiment uses the
single-writer queue already allowed by the Foundation specification.

To reproduce the unsafe call pattern independently, create and seed a fresh
database, then run:

```sh
docker run --rm --platform linux/arm64 \
  --volume "$PWD/artifacts/example:/work" \
  bigbrain-ladybug-spike:v0.17.0 \
  transactions --db /work/example.lbdb --samples 5 --writes 1 \
  --serialize-writes=false
```

The command is expected to terminate with a native `SIGSEGV` for the pinned
pair. Production code must keep all LadybugDB writes behind one queue and must
not assume concurrent prepared calls are safe.

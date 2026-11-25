# BB-0001 LadybugDB technical spike report

## Decision

Proceed with LadybugDB `v0.17.0` and `go-ladybug v0.17.0` for Foundation on
Linux ARM64, with one mandatory constraint: the daemon must serialize writes
through a single queue. The pinned pair passes persistence, transaction,
rollback, crash recovery, migration, and offline distribution checks when
writes are queued.

Two simultaneous prepared writes through separate Go connections reproducibly
crash in native code at `lbug_connection_prepare`. The minimal test terminates
with `SIGSEGV` at address `0x8`. Two thousand concurrent read-only prepared
queries completed without error. Foundation must keep writes serialized and
must retain the unsafe-write reproduction as a regression check when upgrading
LadybugDB or the binding.

The CLI and daemon can share one Go executable with separate modes. The Linux
package must also include `liblbug.so.0.17.0`; the tested distribution is not a
single static binary.

## Tested configuration

| Component | Pin |
| --- | --- |
| Target | Linux ARM64, Debian 12 runtime |
| Builder | Go 1.26 on Debian 12 ARM64 |
| Go binding | `github.com/LadybugDB/go-ladybug v0.17.0` |
| Target engine | LadybugDB `v0.17.0` |
| Target native archive | SHA-256 `a486000f7305cb601e3074eb06026e9eddcf3d7517df7bbc913d2d36e3108121` |
| Migration source | LadybugDB CLI `v0.13.1` |
| Source CLI archive | SHA-256 `8f7f15fe573c836d2de9ac0ad621de9766f127eb7b745fab195aa868cc5e7c1a` |
| Container engine | Docker 29.9.2, Linux ARM64 |

The official `v0.13.1` CLI requires glibc 2.38 and `GLIBCXX_3.4.32`, so its
source/export container uses Debian 13. The `v0.17.0` Go package runs on the
Debian 12 target.

## Results

| Check | Result |
| --- | --- |
| Environment and one-hop graph | Passed. Reopening preserved D1, E1, E2, `D1 -> justified_by -> E1`, and the memory and relationship sources. Forward and reverse traversal matched the fixture. |
| Transactions | Passed with queued writes. Uncommitted data was invisible. Commit became visible. Rollback left zero memory, receipt, and relationship records. Fifty writes completed with no retry or error. |
| Measured latency | Database open was 6,228 µs. One-hop retrieval was 280 µs median and 428 µs p95 over 100 samples. Seventeen of 50 writes waited for the queue, with 1,991 µs p95 queue wait. These are observations, since the specification defines no latency threshold. |
| Crash before commit | Passed. Reopening found zero memory, receipt, and relationship records. Retry created exactly one of each. |
| Crash after commit before acknowledgment | Passed. Reopening found one memory, receipt, and relationship. Retry returned the original ID with `replayed=true` and kept all counts at one. A changed payload returned `IDEMPOTENCY_CONFLICT`. |
| `v0.13.1` to `v0.17.0` migration | Passed. Export/import preserved all IDs, scopes, text, sources, four receipts, relationship direction, weight, and provenance. Replaying `seed-d1` returned D1 without changing its count. |
| Offline package | Passed. A 9.7 MiB archive installed in a fresh Debian 12 ARM64 container with `--network none`, no Go toolchain, and no C compiler. It stored, reopened, and traversed the fixture. Final archive SHA-256 was `4e16595da5675c4ddfd5dedbf68c8325b7c6f065de64ba001764e2051c74fc02`. |

## Distribution contents

The package contains:

- `bin/ladybug-spike`;
- `lib/liblbug.so.0.17.0` and its loader symlinks;
- a package/version manifest;
- the LadybugDB and go-ladybug MIT license notices.

The executable resolves the native library relative to `../lib`. The remaining
dependencies are standard Debian runtime libraries: glibc, `libdl`,
`libpthread`, `libm`, and `libgcc_s`.

## Reproduction

Run `./scripts/run-spike.sh` from this directory with Docker Desktop running on
an ARM64 host. Each run writes fresh, ignored evidence and distribution
directories without replacing earlier runs. See [README.md](README.md) for the
individual experiments and the unsafe concurrency reproduction.

Upstream references: [Go binding](https://github.com/LadybugDB/go-ladybug/tree/v0.17.0),
[target engine release](https://github.com/LadybugDB/ladybug/releases/tag/v0.17.0),
[source engine release](https://github.com/LadybugDB/ladybug/releases/tag/v0.13.1),
[migration procedure](https://docs.ladybugdb.com/migrate/), and
[concurrency model](https://docs.ladybugdb.com/concurrency/).

---
kadai: task
id: "BB-0002"
title: "01 · Foundation"
status: "done"
depends_on: [BB-0001, BB-0017]
---
Deliver the local prototype with a daemon, CLI, persistent storage, project and global scopes, and one-hop associative retrieval.

## Completion criteria

After the daemon restarts, a second agent follows a stored relationship from a decision to its source. The isolation, concurrency, and recovery checks pass with the network disabled.

Complete the subtasks and mark this milestone as `done`.

[Project reference](../../docs/SPEC.md)

<!-- kadai:log:start -->

## Log

### 2026-11-24T22:25:11-03:00

Started Foundation using Go and the qualified LadybugDB 0.17.0 build on Debian 12 ARM64. Writes use a single serialized path and the CLI accepts named flags for replay.

### 2026-11-24T22:32:25-03:00

Completed Foundation and all six subtasks. The Debian 12 ARM64 package passed offline acceptance. A second CLI session reopened the database, retrieved the Orbit decision, and followed its relationship to the source evidence. The package and raw results are available locally under dist/.

### 2026-11-24T22:38:43-03:00

Reviewed and simplified Foundation. Fixed JSON output for help and malformed commands, protected the database mutex on panic, tightened daemon readiness checks, closed native handles on failed initialization, and aligned the client response limit with valid query sizes. Go tests, vet, all 12 offline acceptance checks, and a large-response check passed. Refreshed the local ARM64 package and acceptance results.

<!-- kadai:log:end -->

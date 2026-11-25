---
kadai: task
id: "BB-0008"
title: "Measure transactions and queries with two clients"
status: "done"
parent: BB-0001
depends_on: [BB-0007]
---
Use two connections from the same database object. Interleave reads, short writes, and relationship creation. Store each write, its idempotency key, and its result in one transaction.

## Acceptance criteria

Verify that both commit and rollback leave the database in a consistent state. Record the workload, database opening time, query latency, contention, and any limits observed.

[Project reference](../../docs/SPEC.md#initial-validation)

<!-- kadai:log:start -->

## Log

### 2026-11-24T21:59:11-03:00

Found and reproduced a native concurrency defect before acceptance. Two Go goroutines using separate connections from one Database object crashed in lbug_connection_prepare with SIGSEGV at address 0x8. The minimal reproduction uses one write per connection and is preserved by the transactions command with serialized writes disabled. Because the Foundation daemon may queue writes, the acceptance run will serialize database writes while retaining two connections and will measure queue contention. Reads, manual commit visibility, and rollback checks remain unchanged.

### 2026-11-24T22:00:09-03:00

Passed with the daemon-style single-writer queue. Two connections confirmed that uncommitted data is invisible, committed data becomes visible, and rollback leaves zero Memory, Receipt, and Evidence records. Fifty writes completed with no database retries or errors; 17 waited for the queue, with p95 queue wait 1,940 microseconds. The persisted one-hop query measured 278 microseconds median and 422 microseconds p95 over 100 samples. Unsafe simultaneous prepared writes remain a documented native crash constraint.

### 2026-11-24T22:07:49-03:00

A separate diagnostic completed 2,000 simultaneous read-only prepared queries across two connections without errors. The observed v0.17.0 native crash is therefore scoped by this spike to simultaneous write activity; Foundation must serialize writes and may evaluate read concurrency separately.

<!-- kadai:log:end -->

---
kadai: task
id: "BB-0008"
title: "Measure transactions and queries with two clients"
status: "todo"
parent: BB-0001
depends_on: [BB-0007]
---
Use two connections from the same database object. Interleave reads, short writes, and relationship creation. Store each write, its idempotency key, and its result in one transaction.

## Acceptance criteria

Verify that both commit and rollback leave the database in a consistent state. Record the workload, database opening time, query latency, contention, and any limits observed.

[Project reference](../../docs/SPEC.md#initial-validation)

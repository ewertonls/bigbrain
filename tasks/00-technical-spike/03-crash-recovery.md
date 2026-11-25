---
kadai: task
id: "BB-0009"
title: "Verify crash recovery before and after commit"
status: "done"
parent: BB-0001
depends_on: [BB-0008]
---
Interrupt the process at two points, first before commit and then after commit but before the response. Reopen the database after each interruption and retry the operation with the original key.

## Acceptance criteria

After a pre-commit interruption, the data and receipt are both absent. After a post-commit interruption, both persist, the retry returns the same ID, and committed relationships remain queryable.

[Project reference](../../docs/SPEC.md#initial-validation)

<!-- kadai:log:start -->

## Log

### 2026-11-24T22:00:54-03:00

Passed both intentional process-death checkpoints. Before commit, reopening showed zero memories, receipts, and relationships; retry created exactly one of each. After commit but before acknowledgment, reopening showed one of each; retry returned CRASH-AFTER with replayed=true and kept all counts at one. Reusing the committed key with DIFFERENT-ID returned IDEMPOTENCY_CONFLICT and created no data.

<!-- kadai:log:end -->

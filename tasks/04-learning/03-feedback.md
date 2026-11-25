---
kadai: task
id: "BB-0029"
title: "Apply explicit feedback and a forgetting policy"
status: "done"
parent: BB-0005
depends_on: [BB-0004]
---
Record use in a separate write with its author and source. Update priority, activation count, and last activation time within the project. Require evidence to change confidence and a separate, reviewed global write to change a global weight. Implement an explicit policy that can lower priority offline.

## Acceptance criteria

Repeated reads leave these attributes unchanged. Private feedback in A preserves the result IDs and ordering in B, both with and without global scope included. Applying the forgetting policy lowers the priority of selected material while keeping its evidence queryable.

[Project reference](../../docs/ARCHITECTURE.md#learning-and-the-memory-lifecycle)

<!-- kadai:log:start -->

## Log

### 2026-11-25T14:22:02-03:00

Added explicit relationship use, evidence-backed confidence, reviewed global weights, and an offline forgetting policy. Project feedback on global edges is isolated. Audited writes include author, source and timestamp with chronological pagination. Native tests verified ranking stability in another project, replay, cutoff handling, restart and deletion cleanup.

<!-- kadai:log:end -->

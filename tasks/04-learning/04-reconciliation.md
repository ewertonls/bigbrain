---
kadai: task
id: "BB-0030"
title: "Reconcile conflicting claims and verify how B applies a lesson"
status: "done"
parent: BB-0005
depends_on: [BB-0028, BB-0029]
---
Record supporting and contrary evidence. Link each conclusion to its claims and sources, and explain whether the claims apply in different contexts, are disputed, or supersede one another.

## Acceptance criteria

After A publishes an approved lesson, B applies it and records feedback or a counterexample with its source. The review explains when the lesson applies and lets readers inspect both claims within their scope.

[Project reference](../../docs/ARCHITECTURE.md#learning-and-the-memory-lifecycle)

<!-- kadai:log:start -->

## Log

### 2026-11-25T14:22:02-03:00

Added local reconciliation reviews with scoped supporting and contrary claims, source references, applicability and the direction of supersession. The real CLI and daemon acceptance test verifies that B applies an approved lesson from A, records a witnessed counterexample, and can inspect both sources after restart.

<!-- kadai:log:end -->

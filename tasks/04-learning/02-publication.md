---
kadai: task
id: "BB-0028"
title: "Review and publish a lesson to global scope"
status: "done"
parent: BB-0005
depends_on: [BB-0027]
---
Let the user approve the text and source reference to share. Create a separate global memory and keep the link to private evidence in A.

## Acceptance criteria

Before approval, B cannot find the proposal. After approval, B can query with global scope included to see only the approved text and provenance, then record its own experience applying the lesson.

[Project reference](../../docs/ARCHITECTURE.md#learning-and-the-memory-lifecycle)

<!-- kadai:log:start -->

## Log

### 2026-11-25T14:21:06-03:00

Implemented explicit publication of approved text and public provenance as an independent global lesson. The source proposal and its evidence stay private. Native store and CLI tests verified visibility, rollback, idempotency, deletion, and restart.

<!-- kadai:log:end -->

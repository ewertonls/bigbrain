---
kadai: task
id: "BB-0031"
title: "Verify that one agent can resume another agent’s work"
status: "done"
parent: BB-0006
depends_on: [BB-0005]
---
Integrate clients for two different agents with the v1 contract. Use the same project and retain the idempotency key across invocations when retrying a write.

## Acceptance criteria

One agent records a decision, its supporting evidence, and the relationship between them. In another session, the second agent retrieves the same IDs, sources, and explanation through the daemon. It treats the retrieved text as data, and the query leaves the graph unchanged.

[Project reference](../../docs/ARCHITECTURE.md#phases-and-milestones)

<!-- kadai:log:start -->

## Log

### 2026-11-25T14:35:40-03:00

Added installable Codex and Claude Code skills for a project handoff. The offline acceptance test runs their clients in separate session directories, replays the saved writes across a daemon restart, and verifies the original IDs, sources, directed path and read-only retrieval.

<!-- kadai:log:end -->

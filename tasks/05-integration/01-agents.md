---
kadai: task
id: "BB-0031"
title: "Verify that one agent can resume another agent’s work"
status: "todo"
parent: BB-0006
depends_on: [BB-0005]
---
Integrate clients for two different agents with the v1 contract. Use the same project and retain the idempotency key across invocations when retrying a write.

## Acceptance criteria

One agent records a decision, its supporting evidence, and the relationship between them. In another session, the second agent retrieves the same IDs, sources, and explanation through the daemon. It treats the retrieved text as data, and the query leaves the graph unchanged.

[Project reference](../../docs/ARCHITECTURE.md#phases-and-milestones)

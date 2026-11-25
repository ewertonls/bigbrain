---
kadai: task
id: "BB-0019"
title: "Link project memories to global memories"
status: "todo"
parent: BB-0003
depends_on: [BB-0018]
---
Support project–global relationships in either direction. Each relationship belongs to the project. Apply visibility rules to the relationship and both endpoints in queries, paths, source references, counts, and ranking signals.

## Acceptance criteria

Link a global node to memories in projects A and B. A global-only query sees only the global node. A query in A that includes global sees only A’s link. A query restricted to A cannot see the global node. Direct links between A and B remain invalid.

[Project reference](../../docs/ARCHITECTURE.md#retrieval-and-evaluation)

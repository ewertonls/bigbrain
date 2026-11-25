---
kadai: task
id: "BB-0019"
title: "Link project memories to global memories"
status: "done"
parent: BB-0003
depends_on: [BB-0018]
---
Support project–global relationships in either direction. Each relationship belongs to the project. Apply visibility rules to the relationship and both endpoints in queries, paths, source references, counts, and ranking signals.

## Acceptance criteria

Link a global node to memories in projects A and B. A global-only query sees only the global node. A query in A that includes global sees only A’s link. A query restricted to A cannot see the global node. Direct links between A and B remain invalid.

[Project reference](../../docs/ARCHITECTURE.md#retrieval-and-evaluation)

<!-- kadai:log:start -->

## Log

### 2026-11-24T23:38:51-03:00

Reviewed link ownership in creation, migration and both traversal directions. Updated the read-only snapshot to include relationship ownership. Native tests verified that an inaccessible high-weight link changes neither counts nor provenance. All 12 CLI acceptance checks passed on Linux amd64.

<!-- kadai:log:end -->

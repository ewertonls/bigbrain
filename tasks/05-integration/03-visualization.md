---
kadai: task
id: "BB-0033"
title: "Visualize the graph and inspect why a result was retrieved"
status: "done"
parent: BB-0006
depends_on: [BB-0031]
---
Use the daemon to display memories, relationships, and the path that explains a result. Show each relationship’s stored direction and the direction the query traveled. Let readers inspect visible sources.

## Acceptance criteria

Compare the graph view with the daemon response for A alone, A with global scope included, and global scope alone. Nodes, edges, sources, counts, and paths respect each selection. Browsing the graph leaves the data and activation counters unchanged.

[Project reference](../../docs/ARCHITECTURE.md#phases-and-milestones)

<!-- kadai:log:start -->

## Log

### 2026-11-25T14:35:40-03:00

Added a standalone HTML view of a scoped daemon query. Verified visible node and edge counts, source references and both traversal directions against queries for A, A with global, and global alone. Compared graph data and activation attributes before and after browsing.

<!-- kadai:log:end -->

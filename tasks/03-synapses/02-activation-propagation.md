---
kadai: task
id: "BB-0025"
title: "Bound activation propagation on cyclic graphs"
status: "todo"
parent: BB-0004
depends_on: [BB-0024]
---
Starting from the query candidates, follow visible relationships in either direction. Limit depth and neighbors per node, and control revisits. Rank results using weight, context, and confidence when available.

## Acceptance criteria

A query returns evidence two or more hops away when the limits allow it and omits it when they exclude it. The search terminates on cycles and respects the neighbor limit. Adding a private relationship in A leaves results in B unchanged.

[Project reference](../../docs/ARCHITECTURE.md#memories-and-synapses)

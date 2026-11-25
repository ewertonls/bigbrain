---
kadai: task
id: "BB-0025"
title: "Bound activation propagation on cyclic graphs"
status: "done"
parent: BB-0004
depends_on: [BB-0024]
---
Starting from the query candidates, follow visible relationships in either direction. Limit depth and neighbors per node, and control revisits. Rank results using weight, context, and confidence when available.

## Acceptance criteria

A query returns evidence two or more hops away when the limits allow it and omits it when they exclude it. The search terminates on cycles and respects the neighbor limit. Adding a private relationship in A leaves results in B unchanged.

[Project reference](../../docs/ARCHITECTURE.md#memories-and-synapses)

<!-- kadai:log:start -->

## Log

### 2026-11-25T13:49:36-03:00

Implemented bounded traversal in associative and hybrid queries, including offline fallback. Queries can follow one to three hops and one to ten distinct visible neighbors per node. The defaults preserve the one-hop controls. Verified the CLI on macOS ARM64 and compared its default results with the previous binary. Store tests cover cycles, parallel links, alternative routes and ties, confidence, invalidation, fallback, and project isolation at an intermediate global memory. The full Go test suite, build, and vet passed.

<!-- kadai:log:end -->

---
kadai: task
id: "BB-0026"
title: "Explain and test multi-hop paths"
status: "done"
parent: BB-0004
depends_on: [BB-0025]
---
Include the chosen path and the source of the evidence in the result. For each step, identify the relationship’s endpoints, type, stored direction, traversal direction, and provenance.

## Acceptance criteria

Test a path with both forward and reverse steps and a graph with alternative routes. A reader can reconstruct the traversal from the explanation. Every relationship and endpoint in the explanation is visible in the selected scope.

[Project reference](../../docs/ARCHITECTURE.md#memories-and-synapses)

<!-- kadai:log:start -->

## Log

### 2026-11-25T13:58:37-03:00

Multi-hop query results now include the selected relationships in traversal order. Each step preserves its stored endpoints, traversal direction and provenance. Verified mixed directions, alternative routes, depth-dependent winning prefixes, scope isolation, offline fallback and the one-hop JSON contract through store tests and the native CLI. The transport-size regression, full Go suite, build and vet passed on macOS ARM64.

<!-- kadai:log:end -->

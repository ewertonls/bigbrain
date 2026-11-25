---
kadai: task
id: "BB-0004"
title: "03 · Synapses"
status: "done"
depends_on: [BB-0003, BB-0026]
---
Retrieve evidence several hops away through activation propagation and explain the path taken.

## Completion criteria

A query finds a source more than one hop away, terminates on cyclic graphs, and respects limits and visibility throughout the traversal.

Complete the subtasks and mark this milestone as `done`.

[Project reference](../../docs/ARCHITECTURE.md#memories-and-synapses)

<!-- kadai:log:start -->

## Log

### 2026-11-25T13:58:37-03:00

All three Synapses tasks are complete. Verified that the CLI retrieves evidence up to three hops away and explains its source and every relationship on the selected route. Cycle and neighbor limits, read-only relationship attributes and scope isolation passed the native test suite. The Learning phase can begin.

<!-- kadai:log:end -->

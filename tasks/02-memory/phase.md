---
kadai: task
id: "BB-0003"
title: "02 · Memory"
status: "done"
depends_on: [BB-0002, BB-0022, BB-0023]
---
Deliver the first usable release with typed memories, embeddings, hybrid retrieval, and memory history.

## Completion criteria

A paraphrased query from a real session retrieves the expected memory and its source. Hybrid retrieval finds more matches for paraphrases, preserves direct hits, and keeps rankings isolated by scope.

Complete the subtasks and mark this milestone as `done`.

[Project reference](../../docs/ARCHITECTURE.md#retrieval-and-evaluation)

<!-- kadai:log:start -->

## Log

### 2026-11-24T23:33:28-03:00

Started from Foundation on main. Use the Linux amd64 host for native checks and review each child task before committing it.

### 2026-11-24T23:58:00-03:00

Completed and reviewed all six Memory tasks. Verified a database written by the Foundation binary with the new CLI, including graph provenance and saved-key replay. Native Linux amd64 build, tests, race checks, vet, 14 operational checks and the fixed retrieval evaluation passed. Hybrid preserved direct hits and gained two paraphrased sources. The report records the graph miss, irrelevant results and measured costs.

<!-- kadai:log:end -->

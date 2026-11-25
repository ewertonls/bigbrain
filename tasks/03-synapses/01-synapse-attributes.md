---
kadai: task
id: "BB-0024"
title: "Extend the synapse model while preserving existing relationships"
status: "done"
parent: BB-0004
depends_on: [BB-0003]
---
Add `confidence`, `activation_count`, `last_activated_at`, and `created_at`. Keep weight and confidence independent. Preserve IDs and provenance, and support unknown attribute values when reading older relationships.

## Acceptance criteria

Opening an older database preserves query results in both traversal directions. Repeatedly querying the same relationship leaves its weight, confidence, activation count, and last activation time unchanged.

[Project reference](../../docs/ARCHITECTURE.md#memories-and-synapses)

<!-- kadai:log:start -->

## Log

### 2026-11-25T13:39:20-03:00

Schema v6 adds optional confidence, activation count, last activation time, and creation time to Evidence. Existing links retain their IDs, provenance, and unknown attributes. Verified Foundation migration and Memory v5 project–global traversal in both directions, receipt replay, and read-only queries with recorded activations. The native macOS ARM64 build, Go tests, and vet passed.

<!-- kadai:log:end -->

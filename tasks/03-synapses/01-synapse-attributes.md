---
kadai: task
id: "BB-0024"
title: "Extend the synapse model while preserving existing relationships"
status: "todo"
parent: BB-0004
depends_on: [BB-0003]
---
Add `confidence`, `activation_count`, `last_activated_at`, and `created_at`. Keep weight and confidence independent. Preserve IDs and provenance, and support unknown attribute values when reading older relationships.

## Acceptance criteria

Opening an older database preserves query results in both traversal directions. Repeatedly querying the same relationship leaves its weight, confidence, activation count, and last activation time unchanged.

[Project reference](../../docs/ARCHITECTURE.md#memories-and-synapses)

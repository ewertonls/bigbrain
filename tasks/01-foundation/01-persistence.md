---
kadai: task
id: "BB-0012"
title: "Persist memories, relationships, and write receipts"
status: "todo"
parent: BB-0002
depends_on: [BB-0001]
---
Use the storage approach selected in the spike. Store each memory with a stable ID, scope, text, source, and timestamp. Store each relationship once, with both endpoints in the same scope, and record its type, direction, weight, and source.

## Acceptance criteria

One transaction commits the operation, key, and result. An identical retry returns the original ID. Reusing the key with a different operation, scope, or payload returns a conflict without disclosing the previous request.

[Project reference](../../docs/SPEC.md)

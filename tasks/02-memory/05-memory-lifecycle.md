---
kadai: task
id: "BB-0022"
title: "Implement invalidation, history, and transactional deletion"
status: "todo"
parent: BB-0003
depends_on: [BB-0020, BB-0019]
---
On invalidation, hide the memory and its relationships from ordinary queries. Keep the reason, source, and affected decisions available in history. On deletion, remove the record, its incident relationships, and its index entries. In the same transaction, retain only the idempotency key, request fingerprint, and deleted status in each removed resource’s creation receipt.

## Acceptance criteria

Invalidating E1 keeps D1 discoverable in history with its status unchanged. Delete E1, restart, and retry the requests that created E1 and its relationship. Both return `RESOURCE_DELETED`. Repeat the checks after export and import. Reusing the key with a different payload returns a conflict, and replaying the deletion returns its acknowledgment. Rebuilding the indexes leaves the deleted records absent. A published global generalization survives deletion of its local source.

[Project reference](../../docs/ARCHITECTURE.md#retrieval-and-evaluation)

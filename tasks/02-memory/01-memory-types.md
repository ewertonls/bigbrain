---
kadai: task
id: "BB-0018"
title: "Migrate existing memories to the typed model"
status: "todo"
parent: BB-0003
depends_on: [BB-0002]
---
Add semantic, episodic, procedural, decision, preference, entity, and observation types to the API and storage model. Migrate the Foundation database while preserving older records whose type is unknown.

## Acceptance criteria

Store and retrieve all seven types. Reopen a Foundation database and verify that memories retain their IDs, sources, and relationships, including memories whose type is still unknown.

[Project reference](../../docs/ARCHITECTURE.md#retrieval-and-evaluation)

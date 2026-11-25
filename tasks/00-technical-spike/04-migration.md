---
kadai: task
id: "BB-0010"
title: "Test export and import across versions"
status: "todo"
parent: BB-0001
depends_on: [BB-0008]
---
Export a test database and import it into a new database running the target version. Record both versions and preserve the original database.

## Acceptance criteria

Compare IDs, scopes, sources, relationships, and query results across the two databases. Replaying a write acknowledged before export returns the original ID and leaves the memory count unchanged.

[Project reference](../../docs/SPEC.md#initial-validation)

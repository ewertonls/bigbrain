---
kadai: task
id: "BB-0010"
title: "Test export and import across versions"
status: "done"
parent: BB-0001
depends_on: [BB-0008]
---
Export a test database and import it into a new database running the target version. Record both versions and preserve the original database.

## Acceptance criteria

Compare IDs, scopes, sources, relationships, and query results across the two databases. Replaying a write acknowledged before export returns the original ID and leaves the memory count unchanged.

[Project reference](../../docs/SPEC.md#initial-validation)

<!-- kadai:log:start -->

## Log

### 2026-11-24T22:03:16-03:00

Passed the v0.13.1 -> v0.17.0 export/import migration. The v0.13.1 CLI required Debian 13 because its official ARM64 binary requires glibc 2.38 and GLIBCXX_3.4.32. The imported snapshot preserved D1/E1/E2 IDs, project-a scopes, text, sources, all four receipts, and D1 -> justified_by -> E1 with weight and relationship provenance. Forward and reverse retrieval matched. Replaying the acknowledged seed-d1 request returned D1 with replayed=true and left memory and receipt counts at one.

<!-- kadai:log:end -->

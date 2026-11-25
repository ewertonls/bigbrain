---
kadai: task
id: "BB-0015"
title: "Enforce project and global scopes"
status: "todo"
parent: BB-0002
depends_on: [BB-0012]
---
Store the project ID chosen by the client in configuration shared across worktrees. Require an explicit scope and include global data only on request. Allow the user to save a reviewed global copy with its own source.

## Acceptance criteria

With data in A, B, and the global collection, verify searches, ID lookups, traversal, sources, explanations, and counts from A. B stays hidden from A. Reject links from A to B or to the global collection. Two worktrees on different branches retrieve the same ID and source.

[Project reference](../../docs/SPEC.md)

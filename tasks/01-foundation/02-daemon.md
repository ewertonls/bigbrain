---
kadai: task
id: "BB-0013"
title: "Manage one daemon per physical database"
status: "todo"
parent: BB-0002
depends_on: [BB-0012]
---
Elect one instance per database, open the database exclusively, and restrict the Unix socket to the operating system account. Have clients wait for readiness up to a deadline. During shutdown, decline new requests, finish accepted ones, and confirm that the intended instance has exited.

## Acceptance criteria

Two clients starting simultaneously use the same instance. An accepted write finishes during shutdown, and two consecutive shutdown requests succeed. Handle a stale socket after checking the lock and instance identity. Preserve the files when a database is busy, unreadable, or incompatible.

[Project reference](../../docs/SPEC.md)

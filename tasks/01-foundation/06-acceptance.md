---
kadai: task
id: "BB-0017"
title: "Run the operational checks and cross-session retrieval evaluation"
status: "todo"
parent: BB-0002
depends_on: [BB-0014, BB-0016]
---
Use the CLI offline with the D1/E1/E2 fixture from the specification. Reopen the database and retry the write with its saved key after interruptions before and after commit. Exercise concurrent startup and shutdown. Compare stored records before and after querying text that contains an instruction.

## Acceptance criteria

A second agent retrieves D1 and E1 after restart, and queries leave the data unchanged. Before tuning the ranking, fix six questions and their expected sources. Use three direct-match questions and three that require a relationship. With a limit of three results, associative mode gains at least two hits on relationship-dependent questions and retains all direct hits. Record the number of irrelevant results in both modes.

[Project reference](../../docs/SPEC.md)

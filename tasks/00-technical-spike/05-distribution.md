---
kadai: task
id: "BB-0011"
title: "Test the package on a clean, offline machine"
status: "todo"
parent: BB-0001
depends_on: [BB-0007]
---
Package the prototype and its native dependencies for the selected platform. Assess whether the CLI and daemon can ship in one executable.

## Acceptance criteria

Install the package on a clean machine with no compiler and the network disabled. Write the test data, reopen the database, and retrieve the evidence in one hop. Record which files the package ships and any manual steps.

[Project reference](../../docs/SPEC.md#initial-validation)

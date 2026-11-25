---
kadai: task
id: "BB-0009"
title: "Verify crash recovery before and after commit"
status: "todo"
parent: BB-0001
depends_on: [BB-0008]
---
Interrupt the process at two points, first before commit and then after commit but before the response. Reopen the database after each interruption and retry the operation with the original key.

## Acceptance criteria

After a pre-commit interruption, the data and receipt are both absent. After a post-commit interruption, both persist, the retry returns the same ID, and committed relationships remain queryable.

[Project reference](../../docs/SPEC.md#initial-validation)

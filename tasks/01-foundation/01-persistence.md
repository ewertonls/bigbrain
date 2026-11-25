---
kadai: task
id: "BB-0012"
title: "Persist memories, relationships, and write receipts"
status: "done"
parent: BB-0002
depends_on: [BB-0001]
---
Use the storage approach selected in the spike. Store each memory with a stable ID, scope, text, source, and timestamp. Store each relationship once, with both endpoints in the same scope, and record its type, direction, weight, and source.

## Acceptance criteria

One transaction commits the operation, key, and result. An identical retry returns the original ID. Reusing the key with a different operation, scope, or payload returns a conflict without disclosing the previous request.

[Project reference](../../docs/SPEC.md)

<!-- kadai:log:start -->

## Log

### 2026-11-24T22:28:57-03:00

Implemented a versioned LadybugDB schema and atomic record/link receipts. Native development checks passed for reopened receipts, operation/scope/payload conflicts, and preservation of an unrelated LadybugDB schema. Debian ARM64 verification is pending.

### 2026-11-24T22:32:25-03:00

Debian 12 ARM64 tests passed for transactional records and relationships, stable receipts across reopen, and changed operation, scope, and payload conflicts. The offline interruption checks verified zero rows before an uncommitted retry and exactly one memory and receipt after replay.

<!-- kadai:log:end -->

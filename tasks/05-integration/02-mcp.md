---
kadai: task
id: "BB-0032"
title: "Provide an optional MCP adapter"
status: "done"
parent: BB-0006
depends_on: [BB-0031]
---
Map memory tools to the daemon contract. Preserve explicit scope, idempotency keys for writes, errors, and provenance. Keep direct CLI integration available independently.

## Acceptance criteria

The adapter returns the same IDs, sources, and paths as the equivalent CLI query. Retrying a write with the same key returns the same result. A search scoped to A reveals no data from B.

[Project reference](../../docs/ARCHITECTURE.md#phases-and-milestones)

<!-- kadai:log:start -->

## Log

### 2026-11-25T14:35:40-03:00

Exposed the 13 v1 memory operations over MCP stdio using the official Go SDK. Verified CLI-equivalent scoped results and paths, stable error codes, and write replay with the same key across the MCP and CLI clients.

<!-- kadai:log:end -->

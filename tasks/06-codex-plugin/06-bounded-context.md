---
kadai: task
id: "BB-0040"
title: "Return bounded, cited context and a recall receipt"
status: "done"
parent: BB-0034
depends_on: [BB-0038]
---
Implement a context projection over the [daemon client](../../internal/daemon/client.go) and [v1 query contract](../../internal/protocol/protocol.go). Use explicit query text, checkpoint IDs or the latest fully reviewed handoff for the bound database/project. Without a seed, return an empty result or short binding note. Wrap retrieved text as delimited, potentially stale evidence.

## Acceptance criteria

- The complete emitted context (wrapper, labels, citations and excerpts) fits 8,192 UTF-8 bytes and five memories. Cap each excerpt at 1,200 UTF-8 bytes. Exercise multibyte text, duplicate results and long sources. Keep IDs, scope, type, source reference and truncation visible for every included memory.
- Revalidate active status and visibility when reading stored IDs. Support explicit bootstrap IDs and opting out of cross-session bootstrap. Deleted or invalidated text stays out of new context.
- Persist only identifiers, hashes, counts, byte size, time, fallback and skip reasons in bounded receipts. Preview recomputes against current active data and leaves the context epoch and deduplication state untouched.

<!-- kadai:log:start -->

## Log

### 2026-11-27T02:05:55-03:00

Tested valid UTF-8, truncation, duplicate handling and both independent budgets. A real daemon test confirmed preview writes no receipt and querying an invalidated ID emits no stale text. Simple-code review reused the scoped query contract and atomic JSON replacement with a short file lock.

<!-- kadai:log:end -->

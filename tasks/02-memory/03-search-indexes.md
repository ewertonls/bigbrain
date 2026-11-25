---
kadai: task
id: "BB-0020"
title: "Index text and embeddings within the visible scope"
status: "done"
parent: BB-0003
depends_on: [BB-0018]
---
Evaluate text and vector search in the chosen storage engine. Calculate BM25 statistics over the visible collection and select vector candidates within that scope before applying the result limit. Measure the cost of generating embeddings and building the indexes.

## Acceptance criteria

Compare the returned neighbors with exact vector search over visible records. Keep the query, model, and settings fixed, then add near-duplicate texts and vectors in A. The result IDs and order must stay the same in both B and B with global included. Evaluate per-scope indexes or exact search if needed.

[Project reference](../../docs/ARCHITECTURE.md#retrieval-and-evaluation)

<!-- kadai:log:start -->

## Log

### 2026-11-24T23:42:15-03:00

Reviewed scoped statistics, cache lifetime and vector ordering. Fixed exact nearest-neighbor selection to include negative cosines and avoided retaining empty scopes. Native tests compared known cosine and BM25 values before and after private near-duplicates. Go tests and vet passed. The stock LadybugDB extension probe and measured text/embedding costs are in docs/MEMORY-EVALUATION.md.

<!-- kadai:log:end -->

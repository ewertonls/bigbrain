---
kadai: task
id: "BB-0020"
title: "Index text and embeddings within the visible scope"
status: "todo"
parent: BB-0003
depends_on: [BB-0018]
---
Evaluate text and vector search in the chosen storage engine. Calculate BM25 statistics over the visible collection and select vector candidates within that scope before applying the result limit. Measure the cost of generating embeddings and building the indexes.

## Acceptance criteria

Compare the returned neighbors with exact vector search over visible records. Keep the query, model, and settings fixed, then add near-duplicate texts and vectors in A. The result IDs and order must stay the same in both B and B with global included. Evaluate per-scope indexes or exact search if needed.

[Project reference](../../docs/ARCHITECTURE.md#retrieval-and-evaluation)

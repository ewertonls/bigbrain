---
kadai: task
id: "BB-0021"
title: "Combine text, vector, and graph signals and explain each result"
status: "done"
parent: BB-0003
depends_on: [BB-0019, BB-0020]
---
Combine text and vector candidates with evidence reachable through the graph. Rank results by match quality, context, and relationship weight. Explain why each result was returned and identify its source.

## Acceptance criteria

A paraphrase with no terms in common with the expected memory retrieves its source. If the model or vector index is unavailable, the response reports the fallback and offline text and graph retrieval continue to work.

[Project reference](../../docs/ARCHITECTURE.md#retrieval-and-evaluation)

<!-- kadai:log:start -->

## Log

### 2026-11-24T23:46:10-03:00

Reviewed scoring, one-hop traversal and compatibility with existing v1 clients. Fixed fallback reporting for unknown query words and empty visible vector indexes. Store tests verified source-bearing semantic and reverse graph explanations and equality with the associative control when vectors are unavailable. A native CLI query for “automobile” retrieved “A car needs fuel.” and its Fleet log source with the full GloVe model. Build, Go tests and vet passed.

<!-- kadai:log:end -->

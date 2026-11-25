---
kadai: task
id: "BB-0021"
title: "Combine text, vector, and graph signals and explain each result"
status: "todo"
parent: BB-0003
depends_on: [BB-0019, BB-0020]
---
Combine text and vector candidates with evidence reachable through the graph. Rank results by match quality, context, and relationship weight. Explain why each result was returned and identify its source.

## Acceptance criteria

A paraphrase with no terms in common with the expected memory retrieves its source. If the model or vector index is unavailable, the response reports the fallback and offline text and graph retrieval continue to work.

[Project reference](../../docs/ARCHITECTURE.md#retrieval-and-evaluation)

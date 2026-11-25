---
kadai: task
id: "BB-0023"
title: "Compare hybrid retrieval with the Foundation baselines"
status: "todo"
parent: BB-0003
depends_on: [BB-0021]
---
Before tuning the ranking, fix the questions from real sessions, their expected sources, and the result limit. Use a graph larger than the Foundation fixture, with distractors, multiple projects, and paraphrases that share no terms with the expected memories.

## Acceptance criteria

Compare text-only, text-plus-graph, and hybrid retrieval on the same database. Record hits that include the expected source, irrelevant results, latency, and indexing cost. Hybrid retrieval must gain hits on paraphrases, preserve direct hits, and keep rankings isolated by project. Repeat the text and graph baselines with vector search unavailable.

[Project reference](../../docs/ARCHITECTURE.md#retrieval-and-evaluation)

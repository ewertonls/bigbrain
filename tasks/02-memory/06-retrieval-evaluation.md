---
kadai: task
id: "BB-0023"
title: "Compare hybrid retrieval with the Foundation baselines"
status: "done"
parent: BB-0003
depends_on: [BB-0021]
---
Before tuning the ranking, fix the questions from real sessions, their expected sources, and the result limit. Use a graph larger than the Foundation fixture, with distractors, multiple projects, and paraphrases that share no terms with the expected memories.

## Acceptance criteria

Compare text-only, text-plus-graph, and hybrid retrieval on the same database. Record hits that include the expected source, irrelevant results, latency, and indexing cost. Hybrid retrieval must gain hits on paraphrases, preserve direct hits, and keep rankings isolated by project. Repeat the text and graph baselines with vector search unavailable.

[Project reference](../../docs/ARCHITECTURE.md#retrieval-and-evaluation)

<!-- kadai:log:start -->

## Log

### 2026-11-24T23:33:28-03:00

Fixed the seven resumption questions, CLI search strings, expected sources, relevance sets and top-three limit in tests/memory/fixture.json before implementing search. The spike, Foundation and CLI development sessions supply the evidence. Search strings paraphrase or condense the recorded project questions. The three paraphrase strings share no tokens with their target memories.

### 2026-11-24T23:57:10-03:00

Reviewed the native CLI evaluation and strengthened it to reject duplicate hits and repeat scope checks after a cold reopen. On the preregistered fixture, hybrid retrieved 2/3 paraphrased sources and 3/3 direct sources. Both controls retrieved 0/3 paraphrases. Hybrid missed the graph question and returned 10 irrelevant results. All 84 scope comparisons and 42 missing-model comparisons passed. The report includes per-mode latency, index costs and reproduction steps.

<!-- kadai:log:end -->

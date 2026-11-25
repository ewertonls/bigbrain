---
kadai: task
id: "BB-0016"
title: "Retrieve and explain evidence in one hop"
status: "done"
parent: BB-0002
depends_on: [BB-0015]
---
Implement a text-only control and expand matches along both incoming and outgoing relationships. Limit neighbors per match, prioritize higher weights, deduplicate by ID, preserve the best text match, and break ties consistently. Apply scope checks at every step.

## Acceptance criteria

In the Orbit example, a search for `Orbit` finds the decision and its evidence. A search for `receiver` traverses the relationship in reverse. The explanation identifies the stored direction and the direction traveled, and cites the memory and relationship sources. Removing the relationship eliminates the indirect results.

[Project reference](../../docs/SPEC.md)

<!-- kadai:log:start -->

## Log

### 2026-11-24T22:28:57-03:00

Implemented case-insensitive term matching and deterministic one-hop traversal. The default result limit is ten and each seed contributes at most five distinct neighbors. Local store tests passed for priority, deduplication, and scoped lookups.

### 2026-11-24T22:32:25-03:00

Verified ranked and deduplicated neighbors and explanations in both directions. Without the Orbit relationship, both queries returned only their direct matches. All three direct holdout hits were retained and all three relationship-dependent sources were recovered.

<!-- kadai:log:end -->

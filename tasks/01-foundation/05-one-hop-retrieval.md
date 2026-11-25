---
kadai: task
id: "BB-0016"
title: "Retrieve and explain evidence in one hop"
status: "todo"
parent: BB-0002
depends_on: [BB-0015]
---
Implement a text-only control and expand matches along both incoming and outgoing relationships. Limit neighbors per match, prioritize higher weights, deduplicate by ID, preserve the best text match, and break ties consistently. Apply scope checks at every step.

## Acceptance criteria

In the Orbit example, a search for `Orbit` finds the decision and its evidence. A search for `receiver` traverses the relationship in reverse. The explanation identifies the stored direction and the direction traveled, and cites the memory and relationship sources. Removing the relationship eliminates the indirect results.

[Project reference](../../docs/SPEC.md)

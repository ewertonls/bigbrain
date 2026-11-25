---
kadai: task
id: "BB-0017"
title: "Run the operational checks and cross-session retrieval evaluation"
status: "done"
parent: BB-0002
depends_on: [BB-0014, BB-0016]
---
Use the CLI offline with the D1/E1/E2 fixture from the specification. Reopen the database and retry the write with its saved key after interruptions before and after commit. Exercise concurrent startup and shutdown. Compare stored records before and after querying text that contains an instruction.

## Acceptance criteria

A second agent retrieves D1 and E1 after restart, and queries leave the data unchanged. Before tuning the ranking, fix six questions and their expected sources. Use three direct-match questions and three that require a relationship. With a limit of three results, associative mode gains at least two hits on relationship-dependent questions and retains all direct hits. Record the number of irrelevant results in both modes.

[Project reference](../../docs/SPEC.md)

<!-- kadai:log:start -->

## Log

### 2026-11-24T22:25:18-03:00

Fixed the six English retrieval questions, expected sources, and relevance sets in docs/FOUNDATION-EVALUATION.md before implementing ranking.

### 2026-11-24T22:31:24-03:00

The first offline run found two packaging checks to fix: the ELF loader path contained literal quotes, and Docker exposed inactive tunnel interfaces in the isolated container. The harness passed with the library path supplied explicitly. Corrected the loader flags and checked the route table for external routes, then started a clean rerun with the bundled library path.

### 2026-11-24T22:32:25-03:00

Ran ./scripts/verify-foundation.sh successfully. Go tests and vet passed. All 12 offline checks passed on Debian 12 ARM64 with no Go toolchain or compiler. At a limit of three, text scored 3/3 direct and 0/3 relationship hits, and associative scored 3/3 direct and 3/3 relationship hits. Both modes returned zero irrelevant results. Restart plus first query took 33.256 ms. See docs/FOUNDATION-EVALUATION.md and the local dist/acceptance.json.

### 2026-11-24T22:38:43-03:00

Reran all 12 offline Debian 12 ARM64 checks after the code review. The interrupted-write checks now verify the v1 error and saved-key retry guidance. Both retrieval modes retained their original scores. A separate packaged CLI check returned 48 matches in an 18,585,360-byte JSON response, above the previous client limit.

<!-- kadai:log:end -->

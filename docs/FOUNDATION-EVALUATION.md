# Foundation retrieval evaluation

This fixture was fixed before implementing the ranking. D1, E1, and E2 are
labels for the IDs returned by the CLI. Seed the three memories in the
Foundation specification using their exact text and sources, and link D1 to E1
with `justified_by` and source `Review of Decision 12`.

| Category | Question | CLI search text | Expected memory and source | Relevant results |
| --- | --- | --- | --- | --- |
| Direct | How does Orbit send readings? | Orbit | D1 · Decision 12 | D1, E1 |
| Direct | What does the receiver accept? | receiver | E1 · Test 7 | E1, D1 |
| Direct | What background does the dashboard use? | dashboard | E2 · Note 4 | E2 |
| Relationship | What evidence supports the decision about Orbit? | Orbit | E1 · Test 7 | D1, E1 |
| Relationship | Which decision relies on the receiver's quota? | receiver | D1 · Decision 12 | E1, D1 |
| Relationship | What evidence justifies sending batches? | batches | E1 · Test 7 | D1, E1 |

Run the search text for each question in each mode with a limit of three. Count
a hit when the expected ID and source are both present. Count each returned
memory outside the question's relevance set as an irrelevant result.

## Offline results

| Mode at limit 3 | Direct hits | Relationship-dependent hits | Irrelevant results |
| --- | ---: | ---: | ---: |
| Text | 3/3 | 0/3 | 0 |
| Associative | 3/3 | 3/3 | 0 |

Run `./scripts/verify-foundation.sh` from the repo root. It runs the Go tests
and vet, packages the CLI with its native library, and installs the archive
in a fresh Debian 12 ARM64 container with the network disabled and no
compiler. The script prints the package location and writes
`acceptance.json` with the scores and checks. A separate build provides the
commit barriers used in the interruption tests.

The packaged CLI recovered the Orbit decision and its evidence after restart
in both traversal directions. The control database without the relationship
returned only text matches. Restart plus the first query took 33.256 ms in
the recorded run. Concurrent startup, version checks, graceful shutdown,
crash recovery, scope isolation, worktree access, and read-only retrieval
also passed. An unprivileged rerun passed the same checks and confirmed that
a database with read permission removed retained its checksum after startup
was rejected.

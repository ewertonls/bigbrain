# BigBrain — Phase 1: Foundation

*Proposal for the local, offline prototype.* The [architecture document](ARCHITECTURE.md) describes all five phases of the project.

When an agent returns to a project, it may find a decision without knowing why someone made it. Foundation preserves memories and their relationships across sessions. A second agent should be able to follow a decision back to the evidence behind it, even when the two records use different terms.

| Area | Proposal |
| --- | --- |
| Database | Start with LadybugDB. SQLite was also considered. LadybugDB's suitability for this workload still needs to be measured. |
| CLI and daemon | Evaluate shipping both in one executable. A data command starts the daemon in the background if needed. Subsequent commands reuse the running instance. The user can stop it from the CLI. |
| MVP scope | Store memories locally, independently of the agent or model. A query with a project ID searches only that project by default. Including the global collection requires an explicit choice. The first evaluation uses text and recorded relationships with the network disabled. |
| Prototype choices | Use Go if the binding passes the transaction, crash recovery, and offline distribution tests. Use HTTP over a Unix domain socket and versioned JSON. |
| Open decisions | The first Unix platform and CPU architecture, final CLI syntax, physical schema, and ranking beyond the one-hop experiment. Cypher is a candidate query language. |

## Memories, relationships, and access

A **memory** has a stable ID, a scope, text, a source reference, and the time it was recorded. It can represent a decision, an observation, or a hypothesis. In Foundation, a **relationship** connects two memories in the same scope and records its direction, type, weight, and provenance. A reader can distinguish the source of an observation from the person or agent who linked it to a decision. Weight determines how much priority the relationship gets during retrieval. The source provides the evidence for judging the claim.

The client chooses a project ID once, stores it in configuration shared by the project's agents, and passes it to the CLI. In a Git repository, that configuration can live in the directory shared by all worktrees (`git rev-parse --git-common-dir`). The stored ID is independent of the branch, checkout path, and working directory. If the caller omits the scope, the CLI returns an error asking for one.

**Global** refers to a local collection under the same operating system account. To share a lesson in the prototype, the user reviews the text and source reference that can be shared, then saves a new global memory. The original and its relationships stay in the project. Each project query can explicitly include the global collection.

The operating system account controls access to the database files and local socket. Clients running under that account can select a different project by supplying its ID. The ID selects the working context for a request. The daemon limits searches and each traversal step to the selected scope, including lookups by ID, explanations, content, counts, and source references.

**Retrieved memories are evidence.** The response returns the text in a data field alongside its source and the reason it was retrieved. Queries are read-only, even if the text contains an instruction. Any subsequent action by an agent requires its own authorization. Conflicting claims retain their sources so a reader can examine them side by side.

## Evaluating associative retrieval

The MVP needs three CLI data operations. The first records a memory and returns its ID. The second links two existing IDs and records the source of the relationship. The third queries the selected scope and returns IDs and sources up to the requested result limit. The Memory phase introduces memory types, invalidation, deletion, and relationships between project and global memories.

A text query finds memories that contain the search terms. In associative mode, the daemon explores one hop from each text match along incoming and outgoing relationships. It limits how many neighbors it examines per match and gives higher-weight relationships priority. It deduplicates results by ID and retains the best text match. Each neighbor's explanation reports the relationship in its stored direction and the direction the query traveled (forward or reverse). The first experiment can use a text scan and break ties by ID for stable ordering.

### Cross-session retrieval experiment

An agent records the following fictional data in project A and ends its session.

| ID | Memory | Source |
| --- | --- | --- |
| D1 | Orbit sends readings in batches. | Decision 12 |
| E1 | The receiver accepts only one request per minute. | Test 7 |
| E2 | The dashboard uses a light background. | Note 4 |

The agent links D1 to E1 with the relationship `justified_by` and cites the review of Decision 12 as the source of the link. A second agent searches for `Orbit` with a limit of two results.

- **Text-only control.** Return D1. It is the only record whose text contains the search term.
- **Forward traversal.** Return D1 and E1. The explanation for E1 shows the stored relationship `D1 → justified_by → E1`, forward traversal from D1 to E1, the review of Decision 12, and Test 7. E2 stays out of the results.
- **Reverse traversal.** Query `receiver` with the same limit. Text alone returns E1. Associative mode also returns D1 using the same stored relationship `D1 → justified_by → E1`. The explanation for D1 reports reverse traversal from E1 to D1, the review of Decision 12 as the link source, and Decision 12 as the memory source.
- **Control without the relationship.** Repeat both associative queries on a copy with the same texts and the link removed. Only D1 should appear for `Orbit` and only E1 for `receiver`. Reopening the original database should return both D1 and E1 for each query again.

Before tuning the ranking, set aside six questions an agent might ask when resuming work. Three should have direct text matches and three should require a relationship. Record in advance which memory supports each answer. Compare both retrieval modes on the same database with a limit of three results. Count a hit when the expected memory and its source appear in the top three. Associative mode should gain at least two hits on the relationship-dependent questions and retain all direct hits. Also record how many irrelevant results each mode returns.

## CLI, concurrency, and daemon lifecycle

The CLI sends requests over HTTP through a Unix domain socket restricted to the operating system account. One BigBrain daemon per physical database keeps the database open and serves all clients. LadybugDB documents a single read-write database object with connections derived from it. The daemon owns that object and handles access to the database file. [LadybugDB concurrency](https://docs.ladybugdb.com/concurrency/)

The v1 local protocol and the CLI's JSON output share one contract for agents. Each response includes `version` and either a successful `result` or an `error` with a stable `code` and a diagnostic `message`. Each query result includes the memory ID, scope, text, source, and reason for the match. For a neighbor, the explanation also identifies the path and the source of the relationship. In JSON mode, the CLI reserves standard output for the JSON response.

The caller generates an idempotency key for each write. Before invoking the CLI, the agent saves the key, operation, scope, and payload in its task record. If the response is lost, a later invocation reads that record and resends the same request. Two intentional writes with identical text get distinct keys.

| Situation | Expected behavior |
| --- | --- |
| First data command | The CLI tries to connect to the database's socket. If the daemon is absent, one client wins the startup election and launches the same executable in daemon mode. The others wait. The daemon signals readiness only after it has gained exclusive ownership and opened the database. Clients wait up to a deadline and report a diagnostic if startup fails. |
| Running instance | The CLI checks the database identity and protocol version before sending the operation. On a version mismatch, it returns a stable error code and guidance on aligning the versions. Help and status work while the daemon is stopped. |
| Concurrent writes | The daemon may queue writes. It commits the write, key, and result in one transaction before acknowledging the request. Reusing the key with a different operation, scope, or payload produces a conflict without exposing the previous request. A matching retry returns the previous result while the resource exists. Once Memory introduces deletion, retrying the matching creation request for a deleted resource returns `RESOURCE_DELETED`. |
| Requested shutdown | The daemon declines new requests, finishes accepted ones, closes the database, and releases the socket and exclusive ownership. The CLI confirms shutdown after observing that the specific instance has exited. Clients that arrive during shutdown receive a temporary status. Asking for shutdown again when the daemon is absent succeeds. |
| Crash or stale socket | On startup, the daemon checks the operating system lock and instance identity before removing a stale socket. Once it has exclusive ownership, it reopens the database and checks its integrity. If the database is busy, unreadable, or incompatible, startup reports a diagnostic and preserves the files. A client can retry a write whose outcome is unknown using the original key. |

A configurable idle timeout can use the same shutdown procedure. The timer starts only when the queue is empty and all in-flight operations have finished. Choose the default after measuring startup latency.

### Operational acceptance criteria

1. Start two CLI clients simultaneously against a stopped database. Both commands must finish and use the same daemon instance. Two distinct writes must remain visible after stopping and restarting the daemon.
2. Interrupt the daemon after commit but before it delivers the response. Resend the request from a new CLI invocation with the saved key. It must return the same ID and leave exactly one memory. Changing the text while keeping the key must produce an identifiable JSON conflict code. Repeat with a new write and interrupt the daemon before commit. On reopening the database, both the new memory and its key must be absent.
3. Request shutdown during an accepted write. The write must finish, new requests must receive the shutdown status, and two consecutive shutdown requests must leave the database closed.
4. Create a memory in project B with the same search term as the experiment and a third memory in the global collection. Text searches, traversals, and ID lookups scoped to A may reveal only A. Explicitly including the global collection may return the global memory. B must remain inaccessible. In Foundation, attempts to link A to B or A to the global collection must fail.
5. Store test data containing an instruction to delete a memory. Querying it must return the instruction as data with its source. Compare the stored records and relationships before and after the query. They must be identical.
6. Record D1 in one worktree and query it from another worktree on a different branch using the same project ID. The response must contain D1's ID and original source.

## Initial validation

| Question | Evidence to collect |
| --- | --- |
| Can LadybugDB handle short writes and one-hop queries? | Pin the engine and client library versions. On a disposable database, use two clients to interleave memory writes, relationship creation, and reads. Measure startup and query latency, verify IDs and relationships after a crash, and record any limits encountered. |
| How should the executable be distributed? | LadybugDB's Go API uses its C API. For the first candidate OS and CPU architecture, list the required native libraries and other files. Install the package on a clean machine without a compiler and run the full experiment offline. Reconsider Go if the binding or distribution test fails. [LadybugDB installation](https://docs.ladybugdb.com/installation/#go) |
| How do memories survive an engine upgrade? | Export a test database and import it into a new database, preserving the original. Compare IDs, scopes, sources, relationships, and retrieval results. Resend a write acknowledged before export. The original key must return the original ID and leave the memory count unchanged. [LadybugDB migration](https://docs.ladybugdb.com/migrate/) |

## Transition to the Memory phase

The first usable release targets hybrid retrieval. The one-hop experiment above provides the baseline for evaluating text and vector candidates on real project experiences and larger graphs. LadybugDB's text and vector extensions are candidates for that evaluation. [LadybugDB extensions](https://docs.ladybugdb.com/extensions/)

The [architecture document](ARCHITECTURE.md) describes later phases and their evaluation criteria. Remote synchronization and multi-user access remain open questions.

# BigBrain — Vision and architecture

BigBrain should let agents on different projects learn from earlier experiences and build on each other's knowledge. A decision stays linked to the observation that prompted it. A lesson drawn from that decision can be reviewed, shared, and tested against new evidence.

The [Foundation specification](SPEC.md) defines the first offline experiment. The daemon handles reads and writes, enforces visibility rules, and returns results with their provenance. Memory IDs remain stable across changes of agent, model, or provider.

## Memories and synapses

Each memory has a stable ID, a scope, text, a source reference, and the time it was recorded. In the Memory phase, a type field describes the role of each record. All types share the same logical model.

| Type | What it represents | Example |
| --- | --- | --- |
| Semantic | A fact or reusable knowledge | A quota per time window limits how often readings can be sent. |
| Episodic | An experience in a particular time and context | During the trial, the receiver rejected the second request. |
| Procedural | A technique or procedure | Check the quota window before scheduling requests. |
| Decision | A choice and its rationale | Batch readings to stay within the receiver's quota. |
| Preference | A preference or convention | The team prefers to record decisions in English. |
| Entity | A concept, person, tool, or project that other memories refer to | The receiver for readings from the Orbit project. |
| Observation | An observation or piece of evidence attributed to a source | The trial report records one accepted request per minute. |

A **synapse** is a directed relationship. Foundation records its endpoints (`source` and `target`), type, weight, and provenance. The person or agent who proposed the relationship may differ from the one who produced the evidence. The Synapses phase adds the following attributes.

| Attribute | Meaning |
| --- | --- |
| `confidence` | How strongly the available evidence supports the relationship. A new trial may justify revising it. |
| `activation_count`, `last_activated_at` | The number of activations recorded through explicit feedback and the time of the most recent one. |
| `created_at` | The time the relationship was created. |

Weight determines the relationship's priority during retrieval. An important hypothesis with little supporting evidence can have high weight and low confidence. Feedback on past use may increase the weight. Changes to confidence require identifiable evidence. Queries are read-only in every phase. Recording use or reinforcing a relationship requires a separate write.

Migration from Foundation preserves memory IDs and the provenance of both memories and relationships. Older records remain queryable when their memory type or the new synapse attributes are unknown.

## Scope and publication

A query selects the global scope, a single project, or a project with explicit inclusion of the global scope. The daemon applies this selection before generating candidates and at every step of graph traversal. In Foundation, both endpoints of a relationship belong to the same scope. Starting in Memory, a relationship may also connect a project memory to a global memory in either direction. The relationship belongs to the project. Experiences from different projects can connect only through published global knowledge. Traversal requires both the relationship and its endpoints to be visible.

For example, global memory G can have two outgoing relationships, one to an experience in project A and one to an experience in project B. A query in A that includes the global scope can follow the link from G to A's experience. With the global scope excluded, A sees only its local records. A query in B that includes the global scope cannot see A's link. A global-only query sees G alone. The same visibility rules apply to ID lookups, provenance, explanations, counts, and ranking signals. Adding a private relationship in A must leave both the results and their ordering in B unchanged.

In the Learning phase, an agent can propose a generalization in A, supplying its text, rationale, and sources. The proposal stays in A until the user approves the content and source reference for publication. Approval creates a separate global memory. The link to the private evidence stays in A. Projects that query the global scope receive the approved text and provenance. New experiences may support or challenge the generalization.

In the Orbit trial, the limit of one request per minute is a local observation. The lesson approved for publication might be “Measure the receiver's quota before choosing a batch size.” An agent on another project can apply the procedure and record its own receiver's quota.

## Retrieval and evaluation

Foundation combines term matching with one-hop traversal along explicit relationships. The first usable release, in the Memory phase, targets hybrid retrieval. Within the selected scope, text search and vector similarity produce complementary candidates. Visible relationships lead to associated evidence. Ranking combines these signals with relationship weights and relevance to the query context. Confidence contributes once recorded. Each result identifies its source and explains the text match, semantic similarity, or graph path that led to it. If the vector index or model is unavailable, offline text and graph retrieval remains available and the response reports the fallback.

Evaluate the first usable release with questions from real sessions where an agent resumed earlier work. Include paraphrases that share no search terms with the expected memory, and graphs larger than the Foundation fixture, with distractors and data from several projects. Fix the questions, expected sources, and result limit before tuning the ranking. Compare text search, text search with graph traversal, and hybrid retrieval on the same database. Record how often the top results include the expected memory and its source, how many irrelevant results appear, query latency, and indexing cost. Require more hits on paraphrases, preservation of direct hits, and scope isolation. Repeat the text and associative controls with the vector component unavailable.

## Learning and the memory lifecycle

| Process | Observable effect |
| --- | --- |
| Consolidation | A synthesis references the experiences it draws on. Authorized readers can inspect the originals. |
| Generalization | A local lesson states the conditions under which it applies. Publication review determines what becomes available in the global scope. |
| Reinforcement | Feedback attributed to a successful use may raise a relationship's priority. Repeating a read preserves weight and confidence. |
| Forgetting | An explicit policy may lower the priority of rarely used material. The evidence remains queryable within its scope. |
| Reconciliation | Conflicting claims retain their sources. A review can explain how the claims apply in different contexts, why one is disputed, or why one supersedes another. |

Starting in Memory, **invalidation** removes a memory and its relationships from retrieval candidates and paths. An explicit history query in the authorized scope reveals the reason and source for the invalidation. **Deletion** removes the memory from the active database along with its incident relationships and its entries in text and vector indexes. Reopening the database, rebuilding indexes, or retrying the original write with the same key must preserve the deletion. A published generalization has its own identity and source. Deleting a local source removes the relationship to it. The global record has its own review lifecycle.

## Phases and milestones

| Phase | Acceptance milestone |
| --- | --- |
| 1. Foundation | After a daemon restart, a second agent revisits a decision and finds its evidence in one hop. [SPEC.md](SPEC.md) defines the concurrency, crash recovery, isolation, and distribution tests. |
| 2. Memory | The first usable release distinguishes memory types and uses hybrid search to retrieve the source for a paraphrased query. Relationships between project and global memories respect the selected scope in every operation. |
| 3. Synapses | A query finds evidence more than one hop away and explains the path. Activation propagation respects depth and neighbor limits, terminates on cyclic graphs, and prioritizes results using the available weight, context, and confidence. |
| 4. Learning | An agent in B can apply a lesson proposed in A and approved for publication. Feedback and contrary evidence can be recorded with provenance, preserving the experiences and their review history. |
| 5. Integration | Different agents use the versioned contract to resume the same work. An optional Model Context Protocol (MCP) adapter and a graph visualization follow the daemon's rules for queries and explanations. |

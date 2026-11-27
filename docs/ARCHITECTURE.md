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

Store each relationship once. Retrieval can follow it from either endpoint. For example, `D1 → justified_by → E1` answers both “What evidence supports D1?” and “Which decisions rely on E1?” In the second case, the explanation marks the traversal as reverse and retains D1 as the source and E1 as the target. Both directions use the same weight, provenance, visibility checks, and traversal limits.

Weight determines the relationship's priority during retrieval. An important hypothesis with little supporting evidence can have high weight and low confidence. Feedback on past use may increase the weight. Changes to confidence require identifiable evidence. Queries are read-only in every phase. Recording use or reinforcing a relationship requires a separate write.

Migration from Foundation preserves memory IDs and the provenance of both memories and relationships. Older records remain queryable when their memory type or the new synapse attributes are unknown.

## Scope and publication

The operating system account is the security boundary for the local socket and database files. Any client running under that account can select another project ID. Project scope isolates the working context of a request. Agents sharing an account therefore have access to the same set of projects.

The CLI defaults to the configured directory or Git worktree project for writes and includes global records on reads when no scope flag is given. It sends the resolved scope and visibility to the daemon. A query selects the global scope, a single project, or a project with explicit inclusion of the global scope. The daemon applies this selection before generating candidates and at every step of graph traversal. In Foundation, both endpoints of a relationship belong to the same scope. Starting in Memory, a relationship may also connect a project memory to a global memory in either direction. The relationship belongs to the project. Experiences from different projects can connect only through published global knowledge. Traversal requires both the relationship and its endpoints to be visible.

For example, global memory G can have two outgoing relationships, one to an experience in project A and one to an experience in project B. A query in A that includes the global scope can follow the link from G to A's experience. With the global scope excluded, A sees only its local records. A query in B that includes the global scope cannot see A's link. A global-only query sees G alone. The same visibility rules apply to ID lookups, provenance, explanations, counts, and ranking signals. Adding a private relationship in A must leave both the results and their ordering in B unchanged.

In the Learning phase, an agent can propose a generalization in A, supplying its text, rationale, and sources. The proposal stays in A until the user approves the content and source reference for publication. Approval creates a separate global memory. The link to the private evidence stays in A. Projects that query the global scope receive the approved text and provenance. New experiences may support or challenge the generalization.

In the Orbit trial, the limit of one request per minute is a local observation. The lesson approved for publication might be “Measure the receiver's quota before choosing a batch size.” An agent on another project can apply the procedure and record its own receiver's quota.

## Retrieval and evaluation

Foundation combines term matching with one-hop traversal along explicit relationships. The first usable release, in the Memory phase, targets hybrid retrieval. Within the selected scope, text search and vector similarity produce complementary candidates. Visible relationships lead to associated evidence. Ranking combines these signals with relationship weights and relevance to the query context. Confidence contributes once recorded. Each result identifies its source and explains the text match, semantic similarity, or graph path that led to it. If the vector index or model is unavailable, offline text and graph retrieval remains available and the response reports the fallback.

Evaluate the first usable release with questions from real sessions where an agent resumed earlier work. Include paraphrases that share no search terms with the expected memory, and graphs larger than the Foundation fixture, with distractors and data from several projects. Fix the questions, expected sources, and result limit before tuning the ranking. Compare text search, text search with graph traversal, and hybrid retrieval on the same database. Record how often the top results include the expected memory and its source, how many irrelevant results appear, query latency, and indexing cost. Require more hits on paraphrases, preservation of direct hits, and scope isolation. Repeat the text and associative controls with the vector component unavailable.

### Ranking within a scope

For BM25, compute document count, document frequency for each query term, and average document length over the visible collection. Vector search must draw its candidates from that collection before applying the result limit. Taking the nearest neighbors from a shared index and filtering afterward can discard the best visible match. Weight adjustments and activation counts from private feedback belong to the project, even when the feedback concerns a global relationship. Changing a global weight requires a separate, reviewed global write with its own provenance.

Before choosing LadybugDB indexes for Memory, record the ordered top results for B alone and for B with global included. Keep the query, model, index settings, and global data fixed. In A, add documents containing the query terms and near-duplicate vectors, then repeat both queries. The result IDs and their order must match the baseline. Compare vector candidates against exact search over the visible records to detect lost neighbors. If a shared index fails the isolation check, evaluate per-scope indexes or exact search within the selected scope. Repeat the isolation check with private feedback once reinforcement is available.

## Learning and the memory lifecycle

Learning is a shared responsibility in phase 4. External agents record experiences, evidence, and explicit feedback and may propose lessons directly. BigBrain can also run a learning pass over existing records within a selected scope, even when the contributing agent supplied only observations. BigBrain owns the rules that apply feedback and retention policy, links each derived record to its evidence, and records who or what produced it. A learning pass is a separate write operation.

For example, BigBrain can group repeated receiver failures and propose a procedure for checking quotas. It may use an optional, replaceable inference provider to draft the procedure or identify conflicting claims. Explicit rules can process feedback and apply forgetting policies offline. Inference receives only evidence visible to the pass. A proposal stays in its project until the user approves publication, regardless of whether an external agent or BigBrain produced it.

| Process | Observable effect |
| --- | --- |
| Consolidation | A synthesis references the experiences it draws on. Authorized readers can inspect the originals. |
| Generalization | A local lesson states the conditions under which it applies. Publication review determines what becomes available in the global scope. |
| Reinforcement | Feedback attributed to a successful use may raise a relationship's priority. Repeating a read preserves weight and confidence. |
| Forgetting | An explicit policy may lower the priority of rarely used material. The evidence remains queryable within its scope. |
| Reconciliation | Conflicting claims retain their sources. A review can explain how the claims apply in different contexts, why one is disputed, or why one supersedes another. |

Starting in Memory, **invalidation** removes a memory and its relationships from ordinary retrieval. A history query in the selected scope can still inspect the reason, source, and affected decisions. For example, after E1 is invalidated, its history can list D1 by following the incoming `justified_by` relationship. D1 remains visible with its own status until a review changes it.

**Deletion** removes a memory from the active database along with its incident relationships and entries in text and vector indexes. A published generalization has its own identity and source. Deleting a local source removes the relationship to it. The global record has its own review lifecycle.

In the deletion transaction, replace each removed resource's creation receipt with a tombstone containing only the idempotency key, a fingerprint of the operation, scope, and payload, and the deleted status. Discard the original request body and cached result from the idempotency record. Replaying the matching creation request returns `RESOURCE_DELETED`. Reusing its key with different inputs still returns a conflict. The deletion request has its own key and replays its acknowledgment.

To verify, delete E1, restart, and retry the requests that created E1 and its relationship to D1. Both must return `RESOURCE_DELETED`. Repeat after export and import. Rebuilding the indexes must leave E1 and its relationship absent.

## Phases and milestones

| Phase | Stage | Acceptance milestone |
| --- | --- | --- |
| 1. Foundation | Prototype | After a daemon restart, a second agent revisits a decision and finds its evidence in one hop. [SPEC.md](SPEC.md) defines the concurrency, crash recovery, isolation, and distribution tests. |
| 2. Memory | First usable release | Typed memories support hybrid retrieval of the source for a paraphrased query. Project–global relationships respect the selected scope, and adding private records in another project preserves the results and their order. |
| 3. Synapses | Associative memory | A query finds evidence more than one hop away and explains the path. Activation propagation respects depth and neighbor limits, terminates on cyclic graphs, and ranks by weight, context, and available confidence. |
| 4. Learning | Cognitive system | Given only observations from A, BigBrain proposes a lesson and cites its evidence. After user approval, an agent in B can apply it and record feedback or contrary evidence with provenance. |
| 5. Integration | Ecosystem | Different agents use the versioned contract to resume the same work. An optional Model Context Protocol (MCP) adapter and a graph visualization follow the daemon's rules for queries and explanations. |

# BigBrain

BigBrain stores local memories and the relationships that connect them. A
later session can find a decision and follow it to the original evidence.
The Memory release adds typed records, local hybrid search and scoped history
on LadybugDB 0.17.0. Stop a running daemon before starting the
new binary so it can migrate the database.

## Local development

BigBrain uses the official Go binding and native LadybugDB library, both at
`v0.17.0`. The native archives target macOS 13.3 or newer and glibc-based
Linux on ARM64 and x86-64. Install Go 1.26 or newer, GNU Make, a native C
toolchain, curl, tar, and either `sha256sum` or `shasum`. Use a Go toolchain
for the architecture of your shell. On macOS the Xcode Command Line Tools
supply Make and the compiler.

```sh
git clone https://github.com/ewertonls/bigbrain.git
cd bigbrain
make setup
make build
make test
make run ARGS="--help"
```

`make setup` verifies the release archive's SHA-256 checksum and installs
the header and shared library in `lib-ladybug/`. Subsequent builds and tests
reuse that installation. Go fetches the binding through the regular module
dependency in `go.mod`. This follows [Option 2 in the binding README](https://github.com/LadybugDB/go-ladybug/blob/v0.17.0/README.md#option-2-add-the-compiled-libraries-to-your-project).

The Makefile sets the `system_ladybug` build tag and embeds the checkout's
library path in the executable and test binaries. You can run
`./build/bigbrain` directly. Rebuild after moving the checkout.
`make clean` removes the local executable and keeps the downloaded library.

## Build and install

From the source checkout on a machine with Docker:

```sh
docker build --platform linux/arm64 --target package --output type=local,dest=dist .
```

Copy the resulting `dist/bigbrain` directory to the Linux ARM64 machine and
run `bin/bigbrain` from that directory or add `bin` to your PATH. Keep the
bundled `lib` directory beside `bin`. Git is needed for shared worktree
configuration. With an explicit project ID, the CLI can also run outside Git.

The database defaults to `$XDG_DATA_HOME/bigbrain`, or
`~/.local/share/bigbrain` when the variable is unset. Use `--db DIR` to select
another database directory. The first data operation starts the daemon.
BigBrain creates the directory if needed. An existing directory must be
owned by your account and private to it. `bigbrain status` checks the daemon
and `bigbrain shutdown` waits for it to finish. Startup and runtime diagnostics
are written to `daemon.log` in the database directory.

Run `bigbrain db-path --current-project` to print the absolute path to
`memory.lbug`. You can also select `--project ID` or `--global`. All scopes
in the selected database directory share this file. The command works before
the first write and while the daemon is stopped.

```sh
bigbrain db-path --project orbit
bigbrain --db ./dev-data --json db-path --global
```

JSON output includes the resolved `scope`, `directory`, and `database_file`.

Use `bigbrain help` to list commands. Run `bigbrain query -h` or
`bigbrain help query` to see the query flags, descriptions, and defaults.

## Record and retrieve evidence

```sh
bigbrain project init orbit

bigbrain --json record --current-project --key orbit-decision-12 \
  --type decision --text 'Orbit sends readings in batches.' --source 'Decision 12'
bigbrain --json record --current-project --key orbit-test-7 \
  --type observation --text 'The receiver accepts only one request per minute.' --source 'Test 7'

# Substitute the two IDs returned by record.
bigbrain --json link --current-project --key orbit-review-12 \
  --from DECISION_ID --to EVIDENCE_ID --type justified_by --weight 1 \
  --source 'Review of Decision 12'

bigbrain shutdown
bigbrain --json query --current-project --text Orbit --limit 2
```

The optional `record --type` accepts `semantic`, `episodic`, `procedural`,
`decision`, `preference`, `entity`, or `observation`. A query returns
`type: "unknown"` for an untyped memory. Opening a Foundation database
migrates it in one transaction and preserves IDs, sources, links and
previously issued write receipts.

When creating a link, use `--weight` for retrieval priority and optional
`--confidence` for the strength of the supporting evidence, from 0 to 1.
Omitting confidence leaves it unknown. Query and history paths expose
`confidence`, `created_at`, `activation_count` and `last_activated_at` when
known. A new link records its creation time and starts with zero activations.
Older links omit attributes that were never recorded. Reading a link leaves
its attributes intact.

`project init` stores the chosen ID in the Git common directory, so separate
worktrees share it. Every data command requires a scope. Use
`--project ID` for a specific project or `--global` to select the global
collection. To publish reviewed information, record a new global memory with
the approved text, its own source, and a fresh key. A project query includes
global memories when passed `--include-global`. Use the project scope to
link a project memory to a global memory in either direction. The project
owns the relationship. Both endpoints and the relationship must be visible
for a query to return its path, source or ranking contribution.

Use `--id ID` for an exact lookup. The CLI defaults to hybrid retrieval
with a limit of ten results. Use `--mode text` for text matches alone or
`--mode associative` to list text matches first, followed by related
memories. A v1 request that omits `mode` uses associative retrieval.

In associative and hybrid modes, `--max-depth` sets how many relationships
to follow (1–3, default 1). `--max-neighbors` limits distinct neighbors per
node (1–10, default 5). Both directions are eligible. Each step considers
only visible relationships and active memories, and skips memories already
on that path before applying the neighbor limit. At depth 1, associative
mode retains the Foundation response format. It orders indirect matches by
link priority, adjusted for confidence when known, and omits `graph_score`.

```sh
bigbrain --json query --current-project --mode associative --text Orbit \
  --max-depth 3 --max-neighbors 2
```

For associative queries at depth 2 or 3, the starting score is normalized
BM25. Hybrid queries start from the combined text and vector score at any
depth. Each hop multiplies the previous score by `0.8 × weight / (1 + weight)`.
Known confidence adds a factor of `(1 + confidence) / 2`. Unknown confidence
uses a factor of 1. A zero-weight link contributes no activation in hybrid
mode and remains traversable in associative mode.

Each destination keeps its strongest graph contribution. Equally strong
paths to the same memory favor fewer hops, then the lexicographically
smallest sequence of relationship IDs. For a one-hop match, the relationship
is in `reason.path`. For a two- or three-hop match, `reason.steps` lists the
relationships in traversal order, `reason.hops` gives their count, and
`reason.path` equals the last step. Each step identifies the relationship,
its type, stored endpoints, traversal direction, and source.

For example, with stored links `A → B ← C`, a search from A can reach C
through B. Its explanation contains these two steps.

| Step | `from_id` | `to_id` | `type` | `direction` | `source` |
| --- | --- | --- | --- | --- | --- |
| 1 | A | B | justified_by | forward | Decision review |
| 2 | C | B | corroborates | reverse | Lab review |

The second step goes from B to C while preserving the stored endpoints
`C → B`. The match's top-level `source` identifies the evidence for memory C,
such as `Trial 7`. The two step sources identify the reviews that recorded
the relationships.

## Offline hybrid search

Configure the daemon with a local word2vec text vocabulary. The file begins
with the word count and dimension, followed by one word and its vector per
line. Plain text and gzip are supported. For each word used in an embedding,
BigBrain tries the original spelling first, then its lowercase form.
The evaluation used Stanford GloVe
Wiki/Gigaword, 400,000 words and 50 dimensions. Download it once during
setup. The daemon reads the local file when it first receives a hybrid query.

```sh
mkdir -p build/models
curl -fL -o build/models/glove-50.gz https://github.com/piskvorky/gensim-data/releases/download/glove-wiki-gigaword-50/glove-wiki-gigaword-50.gz
echo '5c55f98957aa9fed8d2ac5fb1dcff57af3b23c5a3ee7af3f7945f8d49198eb24  build/models/glove-50.gz' | sha256sum -c -
export BIGBRAIN_EMBEDDINGS="$PWD/build/models/glove-50.gz"
bigbrain shutdown
bigbrain --json query --current-project --mode hybrid --text 'shipping binaries' --limit 3
```

The result includes the memory source and a `reason` with the contributing
BM25 terms and score, cosine similarity, or strongest visible graph contribution.
A path identifies its stored direction, traversal direction, owner, weight
and source. The final score combines normalized BM25, cosine and the best
weighted graph contribution. Ties break by memory ID. The `count` includes
the unique candidates before the output limit.

If the model is missing, unreadable, or has no usable embedding, hybrid
returns associative results with the same depth and neighbor limits and a
machine-readable `fallback` reason.
Start the daemon with `BIGBRAIN_VECTOR_INDEX=off` to disable the vector
index. Restart it after changing its environment or vocabulary. All searches
run locally. [GloVe model and license](https://nlp.stanford.edu/projects/glove/)

## Propose a project lesson

`consolidate` creates a `lesson_proposal` in the project selected with
`--project ID` or `--current-project`. Pass
`--evidence ID` once for each of 1–100 distinct original memories. Any
original memory type is eligible while active in that project. The originals
remain queryable, and each has a `derived_from` link from the proposal.

To generate an extractive proposal locally, give BigBrain the evidence IDs.
It takes the first sentence or line from each memory, up to 240 bytes, and
groups matching excerpts, listing the most frequent first. The producer
is `bigbrain:extractive-v1`.
The following examples use the decision and observation recorded above.

```sh
bigbrain --json consolidate --project orbit --key orbit-quota-synthesis \
  --evidence DECISION_ID --evidence EVIDENCE_ID
```

For a proposal drafted by a person or external agent, supply `--text`,
`--applies-when`, `--rationale` and `--producer` together. The optional
`--source` gives the proposal its own source reference and defaults to the
producer.

```sh
bigbrain --json consolidate --project orbit --key orbit-quota-proposal \
  --evidence DECISION_ID --evidence EVIDENCE_ID \
  --text 'Measure the receiver quota before choosing a batch size.' \
  --applies-when 'A receiver limits the number of requests per minute.' \
  --rationale 'The measured quota determined the batch interval in Orbit.' \
  --producer 'Orbit reviewer' --source 'Review 17'

bigbrain --json history --project orbit --id PROPOSAL_ID
```

Use the returned proposal ID for `history`. Its `proposal` field records
`origin` (`extractive` or `external`), `producer`, `applies_when` and
`rationale`. In `related`, entries with `path.type` set to `derived_from`
and `path.direction` set to `forward` identify the original memories and
their source references. Only `consolidate` creates these outgoing links on a proposal.
Both the proposal and its links stay private to the project. Deleting an
original removes its link. Any excerpt already copied into the proposal
remains part of the proposal's text.

## Invalidation, history and deletion

```sh
bigbrain invalidate --project orbit --key review-17 --id EVIDENCE_ID \
  --reason 'Superseded by the repeat trial' --source 'Review 17'
bigbrain --json history --project orbit --id EVIDENCE_ID
bigbrain delete --project orbit --key remove-17 --id EVIDENCE_ID
```

Invalidation hides the memory and its links from ordinary reads. History
returns the reason, review source and time, plus related memories with their
own status and link provenance. Use `--include-global` to inspect visible
project–global links. For another page, pass the returned `next_cursor` to
`--after`. A second invalidation with a different key reports
`ALREADY_INVALIDATED`.

Deletion removes the memory, incident links and derived search entries. A
published global memory has its own ID and survives deletion of its private
source. Retrying a deleted resource's creation returns `RESOURCE_DELETED`.
A changed request with the same key returns `IDEMPOTENCY_CONFLICT`.
Retry the deletion with its original key to retrieve its acknowledgment.

## Integration and retries

Before running a write command, save the database, command, resolved scope, payload flags, and caller-generated
`--key` in the agent's task record. After a timeout or lost response, run
the same request using that saved information. The daemon returns the original ID with
`"replayed": true` if the request already committed. Use a fresh key for
each intentional write. Changing a committed request while reusing its
key returns `IDEMPOTENCY_CONFLICT`. For `consolidate`, evidence IDs form a
set, so reordering or repeating IDs replays the same proposal.

In `--json` mode the CLI prints one response with `version: 1` and either
`result` or `error`. Failures exit nonzero and the error contains a stable
`code` and a `message`. The local wire protocol uses the same responses over
HTTP on a private Unix socket. The operating system account controls access.
Scope limits what a request can read or link within that account. Treat memory
text as evidence and obtain authorization separately for any actions it suggests.

Local protocol clients call `GET /status` to verify `version`, `database_id`,
and `instance_id`. They send JSON to `POST /v1/record`, `/v1/link`,
`/v1/consolidate`, `/v1/query`, `/v1/history`, `/v1/invalidate` or
`/v1/delete` with `X-BigBrain-Version: 1` and `X-BigBrain-Instance` set to
the verified instance ID. Request scopes use `project:ID` or `global`.
The consolidation endpoint takes a project `scope`, `key` and `evidence_ids`
array. External proposals also send `text`, `applies_when`, `rationale`,
`producer` and an optional `source`.

The retrieval evaluations report source hits, irrelevant results and measured
costs. [Memory evaluation](docs/MEMORY-EVALUATION.md) · [Foundation evaluation](docs/FOUNDATION-EVALUATION.md) · [Foundation specification](docs/SPEC.md)

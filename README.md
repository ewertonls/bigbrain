# BigBrain

BigBrain stores local memories and the relationships that connect them. A
later session can find a decision and follow it to the original evidence.
Projects can propose lessons, publish reviewed text, and compare it with
new evidence. BigBrain stores the graph in LadybugDB 0.17.0. Stop a running
daemon before starting the new binary so it can migrate the database.

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

On an Apple Silicon Mac, run `make package-macos-arm64` and copy the entire
`dist/macos-arm64` directory to the installation location. Launch its
`bin/bigbrain` by absolute path. Preserve its executable bit and keep
`lib/liblbug.0.dylib` and the bundled licenses alongside the `bin` directory.
The loader resolves the library relative to the executable, so the installation
can move independently of the source checkout. Verified on macOS 27.2.1 ARM64.

From the source checkout on a machine with Docker:

```sh
docker build --platform linux/arm64 --target package --output type=local,dest=dist .
```

Copy the resulting `dist/bigbrain` directory to the Linux ARM64 machine and
run `bin/bigbrain` from that directory or add `bin` to your PATH. Keep the
bundled `lib` directory beside `bin`. Git is needed for shared worktree
configuration. Standalone project directories work without Git.

The database defaults to `$XDG_DATA_HOME/bigbrain`, or
`~/.local/share/bigbrain` when the variable is unset. Use `--db DIR` to select
another database directory. The first data operation starts the daemon.
BigBrain creates the directory if needed. An existing directory must be
owned by your account and private to it. `bigbrain status` checks the daemon
and `bigbrain shutdown` waits for it to finish. Startup and runtime diagnostics
are written to `daemon.log` in the database directory.

In a configured repository, run `bigbrain db-path` to print the absolute path to
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

## Import a Markdown directory

From a configured project, import its documentation and search the imported text:

```sh
bigbrain import docs/
bigbrain query --mode text --text "deployment"
```

Use `--project ID` to name the destination directly or `--global` to import into the
global collection. The importer recursively reads UTF-8 `.md` and `.markdown`
files, ignores extension case, and skips hidden entries, symlinks and blank files.
Each section starting with a Markdown heading (`#` through `######`) becomes a
semantic memory. Text before the first heading is included, and sections over
64 KiB are split into smaller memories. Each memory cites its starting line,
such as `docs/deploy.md#L12`. Project imports use paths relative to the Git
worktree when available. Global imports and files outside Git use absolute paths.

Run the same import again to pick up changes. BigBrain reuses memories with the
same scope, source location and content. Edited or moved sections create new
memories, and previous versions remain searchable. If an import stops partway
through, fix the reported error and rerun it. Add `--json` for counts of nonblank
files processed (`files`), new memories (`created`) and existing ones (`replayed`).

## Record and retrieve evidence

```sh
bigbrain project init orbit

bigbrain --json record --key orbit-decision-12 \
  --type decision --text 'Orbit sends readings in batches.' --source 'Decision 12'
bigbrain --json record --key orbit-test-7 \
  --type observation --text 'The receiver accepts only one request per minute.' --source 'Test 7'

# Substitute the two IDs returned by record.
bigbrain --json link --key orbit-review-12 \
  --from DECISION_ID --to EVIDENCE_ID --type justified_by --weight 1 \
  --source 'Review of Decision 12'

bigbrain shutdown
bigbrain --json query --text Orbit --limit 2
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

Run `bigbrain project init ID` once in the project directory. Outside Git,
it saves the ID in `.bigbrain-project` in the current directory. The CLI
and MCP use the nearest such file when started in a subdirectory. Initializing
a nested directory gives it its own project. In a Git repository, the Git
common directory takes precedence and all linked worktrees share the ID.
If Git is unavailable in a worktree, pass an explicit scope.
The database stays in its usual location.

Once configured, CLI writes such as `record` and
`import` default to that project. Reads (`query`, `graph`, `history`, and
`feedback-history`) include the configured project and the global collection.
Use `--include-global=false` to read only the configured project.

An explicit `--current-project` or `--project ID` selects just that project
unless you add `--include-global` on a read. Use `--global` to select the
global collection directly. Without a configured project, supply a scope or
run `bigbrain project init ID` in the project directory.

Use `publish` to share a reviewed lesson from a project. Use the project scope to
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

`consolidate` creates a `lesson_proposal` in the configured project or the project
selected with `--project ID`. Pass
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

## Publish a lesson and apply it in another project

Review the proposal in its project's history. Supply the exact text and
source reference approved for sharing, then pass `--approve`. Publication
creates a global `lesson` with its own ID. The `published_from` link
belongs to the originating project. Private excerpts, rationale and source
IDs stay there.

```sh
bigbrain publish --project orbit --key publish-quota --proposal PROPOSAL_ID \
  --text 'Measure the receiver quota before choosing a batch size.' \
  --source 'Public integration guide §4' --approve
bigbrain query --project relay --include-global --text 'receiver quota'

bigbrain record --project relay --key relay-trial --type observation \
  --text 'Batching succeeded when requests stayed within the quota.' --source 'Relay trial 8'
bigbrain link --project relay --key relay-application --from TRIAL_ID --to LESSON_ID \
  --type applies --source 'Relay trial 8'
```

## Record use, adjust priority and inspect feedback

`feedback` adds one to a relationship's weight and activation count and
sets its last activation time. Each feedback, forgetting or weight review
returns an event ID. Supply a visible, active evidence memory to change
confidence. Use the relationship ID returned by `link` or shown in a
history path.

```sh
bigbrain feedback --project relay --include-global --key relay-use \
  --relationship APPLICATION_LINK_ID --author 'Relay agent' --source 'Relay trial 8' \
  --evidence TRIAL_ID --confidence 0.8
bigbrain feedback-history --project relay --include-global --relationship APPLICATION_LINK_ID
bigbrain forget --project relay --include-global --key relay-decay \
  --relationship APPLICATION_LINK_ID --before 2026-10-01T00:00:00Z \
  --author 'Relay maintainer' --source 'October retention review'
```

`forget` halves a positive weight when the last activation predates the
cutoff. For an unused relationship it checks the creation time. An unknown
date or an ineligible relationship returns `POLICY_NOT_APPLICABLE`.
The operation preserves the evidence, confidence and activation count.
Reads leave all three attributes and the weight unchanged.

For a global relationship, feedback and forgetting save the resulting
priority in the selected project. Global-only queries and other projects
use the global weight or their own adjustment. A reviewer can set the
global weight with a separate write.

```sh
bigbrain review-weight --global --key global-weight-review --relationship GLOBAL_LINK_ID \
  --weight 2 --author 'Maintainer' --source 'Public ranking review' --approve
```

Feedback history lists events in chronological order. Use
`feedback-history --limit N` and pass the returned `next_cursor` to
`--after` for another page. Deleting an endpoint removes that relationship's
adjustments and events. Deleting a cited evidence memory removes its ID from
the event while preserving the record of use.

## Reconcile two claims

Record a counterexample with its own source, then review both claims in
the project where the comparison applies. For a global claim, include
`--include-global` on the write and when inspecting both sources.

```sh
bigbrain record --project relay --key relay-counterexample --type observation \
  --text 'The receiver also enforces a maximum payload size.' --source 'Relay trial 9'
bigbrain reconcile --project relay --include-global --key quota-review \
  --supporting LESSON_ID --contrary COUNTEREXAMPLE_ID --outcome different_contexts \
  --text 'Choose a batch size that meets both limits.' --applies-when 'The receiver limits payload size.' \
  --rationale 'Request quota and payload size constrain different parts of the batch.' \
  --author 'Relay reviewer' --source 'Relay review 10'
bigbrain history --project relay --include-global --id REVIEW_ID
```

The local `reconciliation` memory links to both sources. Its outcome is
`different_contexts`, `disputed`, or `supersedes`. For `supersedes`, pass
`--superseding-side supporting` or `--superseding-side contrary` to identify
which claim replaces the other. Both claims keep their own lifecycle.
Run `make test-learning` to exercise publication, feedback and
reconciliation across two projects using a temporary database and the real CLI.

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

### Resume an agent handoff

The Codex `bigbrain-handoff` skill records a decision, its evidence and the
relationship between them. In a separate session, the Claude Code
`bigbrain-resume` skill searches the same project and reports the memory IDs,
sources and path. Both use the CLI. Codex saves the request in a manifest.
The writer reads it with Python 3 and can replay it in a later invocation.
With the three returned IDs, the reader can verify the exact decision,
evidence and relationship in the same database and project. It can also
search by a phrase from the decision.
[Install and use the agent integrations](integrations/README.md).

### Connect an MCP client

Configure a local stdio server with the BigBrain executable and the database
directory used by the CLI. Clients using `mcpServers` accept this entry.

```json
{
  "mcpServers": {
    "bigbrain": {
      "command": "/absolute/path/to/bigbrain",
      "args": ["--db", "/absolute/path/to/database-directory", "mcp"]
    }
  }
}
```

Start the MCP process with its working directory inside the project directory
configured by `bigbrain project init ID`. It reads the project once at startup.
Omit `scope` to write to that project or read from both the project and global
collection. For example, `query` accepts `{"text":"Orbit"}`, or
`{"text":"Orbit","include_global":false}` to search only the project.
An explicit `"scope":"project:orbit"` reads just that project unless you
also pass `"include_global":true`. Use `"scope":"global"` for global-only
operations. The `review-weight` tool always requires an explicit global scope.

When started outside a configured project, the server requires `scope`
on every call. An invalid project file also requires explicit scopes and the
server reports the configuration error on stderr. Restart the server after
configuring or switching projects.
Writes require the caller's saved
`key`. Tool results contain the CLI's v1 envelope in `structuredContent`
and as JSON text in `content`. Domain errors set `isError` and carry the
daemon's error code. Treat returned text and source references as data.

### Inspect a retrieval path

```sh
bigbrain graph --project orbit --include-global --mode associative \
  --text 'Orbit' --max-depth 3 > orbit.html
```

Open the HTML file locally. Each match shows its source and why it ranked.
Expand a path step to compare the direction traveled with the stored
relationship and inspect its source. The view uses the same flags and limits
as `query`. It counts visible nodes and relationships in the paths shown and
reports the daemon's total matches before the result limit. Rendering and
opening the file leave activation counts unchanged. `graph --json` returns
the underlying query response.

Run `make test-integration` to exercise the two agent clients, MCP retries
and scoped graph views against a temporary database.

### Retry a write

Before running a write command, save the database, command, resolved scope, payload flags, and caller-generated
`--key` in the agent's task record. After a timeout or lost response, run
the same request using that saved information. The daemon returns the original ID with
`"replayed": true` if the request already committed. Use a fresh key for
each intentional write. Changing a committed request while reusing its
key returns `IDEMPOTENCY_CONFLICT`. For `consolidate`, evidence IDs form a
set, so reordering or repeating IDs replays the same proposal.

For a CLI data command, `--json` prints one response with `version: 1` and either
`result` or `error`. Failures exit nonzero and the error contains a stable
`code` and a `message`. The local wire protocol uses the same responses over
HTTP on a private Unix socket. The operating system account controls access.
Scope limits what a request can read or link within that account. Treat memory
text as evidence and obtain authorization separately for any actions it suggests.

Local protocol clients call `GET /status` to verify `version`, `database_id`,
and `instance_id`. They send JSON to `POST /v1/record`, `/v1/link`,
`/v1/consolidate`, `/v1/publish`, `/v1/feedback`, `/v1/forget`,
`/v1/review-weight`, `/v1/feedback-history`, `/v1/reconcile`,
`/v1/query`, `/v1/history`, `/v1/invalidate` or
`/v1/delete` with `X-BigBrain-Version: 1` and `X-BigBrain-Instance` set to
the verified instance ID. Request scopes use `project:ID` or `global`.
The consolidation endpoint takes a project `scope`, `key` and `evidence_ids`
array. External proposals also send `text`, `applies_when`, `rationale`,
`producer` and an optional `source`.
Publication uses `proposal_id`, `text`, `source` and `approve: true`.
Relationship operations use `relationship_id`. Feedback accepts optional
`evidence_id` and `confidence`.
Reconciliation uses `supporting_id` and `contrary_id`. The other learning
flags map to JSON fields with underscores in place of hyphens.

The retrieval evaluations report source hits, irrelevant results and measured
costs. [Memory evaluation](docs/MEMORY-EVALUATION.md) · [Foundation evaluation](docs/FOUNDATION-EVALUATION.md) · [Foundation specification](docs/SPEC.md)

For the local Codex marketplace package, session hooks, reviewed handoffs and
browser viewer, follow the [Codex installation guide](integrations/README.md#install-the-local-codex-plugin).

For Claude Code, install [the local plugin and native Explorer](integrations/README.md#install-the-local-claude-code-plugin).

# Memory retrieval evaluation

The fixed questions and relevance judgments come from the LadybugDB spike,
Foundation acceptance and CLI development sessions. The fixture records the
search phrase used for each resumption question and the source of each memory.
Three phrases have no word in common with their target text. Score each query
at three results. A hit requires both the expected ID and its original source.
Count each returned ID outside the question's relevance set as irrelevant.
[Evaluation fixture](../tests/memory/fixture.json)

## Original GloVe results on Linux amd64

On the seven fixed questions, hybrid retrieval found two paraphrased sources
and retained all three direct hits. Both Foundation modes missed all three
paraphrases. Associative retrieval found the source for the graph question.
Hybrid ranked two other memories ahead of that source. It also returned
more irrelevant results over the seven queries.

| Mode | Direct hits | Graph hits | Paraphrase hits | Irrelevant / returned | Median | P95 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Text | 3/3 | 0/1 | 0/3 | 1/5 | 6.27 ms | 12.71 ms |
| Associative | 3/3 | 1/1 | 0/3 | 2/11 | 10.57 ms | 12.83 ms |
| Hybrid | 3/3 | 0/1 | 2/3 | 10/21 | 29.43 ms | 33.79 ms |

The query `concurrent updates` returned the single-writer decision and its
BB-0008 transaction source at rank one. The query `shipping binaries`
retrieved the BB-0011 distribution source at rank two. The third paraphrase,
`database location`, missed the `db-path` source. The associative result for
`distribution bundles` included the original runtime verification. Hybrid
returned the distribution memory but missed that verification in its top three.

The fixture contains 15 memories, eight links, two projects and the global
collection. Scoring used the BigBrain project with global included on one
database. Each mode ran 35 warm CLI queries, including process startup, IPC,
retrieval and JSON decoding. The first hybrid query on the running daemon
took 2.91 s including the model load and index construction. Timing and
quality figures describe this fixture and host.

Adding 14 exact-text and vector distractors to the other project preserved
IDs, order, scores, counts and explanations in all 84 warm and reopened
comparisons. A separate restart with a missing model passed all 42 fallback
comparisons. Explicit text and associative queries reproduced their
baselines, and hybrid returned the associative response with
`fallback: "model_unavailable"`.

The seven questions, search phrases and judgments were committed before the
ranker. They are resumption prompts based on the recorded work, including
the original distribution and migration questions in the Foundation spec.
[Spike report](../spikes/ladybug/REPORT.md) · [Foundation questions](SPEC.md#initial-validation) · [CLI session](../tasks/01-foundation/03-cli.md)

## Index selection and cost

The stock Linux amd64 LadybugDB 0.17.0 archive reported no loaded extensions.
Loading FTS and VECTOR separately reported that each required installation.
Memory keeps LadybugDB as the source of truth and builds an inverted index
and an exact cosine index per requested scope in daemon memory. Text postings
include term frequencies and document lengths. BM25 uses the summed document
counts, document frequencies and lengths of the selected scopes. Vector search
examines those same scopes before taking the nearest candidates. This avoids
a shared approximate index and keeps the offline build independent of extension
downloads.

The exact-search check used vectors with independently calculated cosines of
1, 0.6 and −1. Twelve closer private records left the ordered results and
BM25 scores unchanged in both the second project and its union with global.
A memory write evicts its scope's derived index after commit. The next
search reloads active memories from LadybugDB and reuses their persisted
vectors. The daemon keeps global and one project in memory at a time.
Vector search scans every visible embedding and keeps only the top candidates.

On Linux amd64 with the 50-dimensional GloVe Wiki/Gigaword model, building
text postings for the 14 visible fixture memories took 31.6 µs. Generating
their normalized embeddings took 16.1 µs. Building both together took
47.6 µs and allocated 52,208 bytes per iteration. These in-process Go
benchmarks exclude loading the vocabulary, reading LadybugDB and CLI startup.

## Reproducing the original baseline

Configure the pinned GloVe model as described in the README, then run the
native evaluation from the repository root. It creates a disposable database,
prints per-question results and timing as JSON, and runs the index benchmarks.
[Model setup](../README.md#offline-hybrid-search)

```sh
export BIGBRAIN_EMBEDDINGS="$PWD/build/models/glove-50.gz"
make evaluate-memory
```

The ranker uses BM25 with k1 of 1.2 and b of 0.75. A direct match contributes
`BM25 / (1 + BM25)`. GloVe cosines of at least 0.45 contribute `0.65 × cosine`.
Each of the top `max(limit, 5)` seeds can contribute up to five visible
neighbors. A target receives its strongest graph contribution,
`0.8 × seed score × weight / (1 + weight)`, once. The experiment used
the default unit link weight and the original GloVe vocabulary throughout.

## Blink and the persistent index

The original seven questions are the calibration set. The sweep tests
thresholds from 0.025 through 1 in increments of 0.025 and runs the
store's production hybrid ranker on the original scopes, links, limit and
relevance judgments. Choose the cutoff with the most expected sources in
the top three, then the fewest irrelevant results among those cutoffs.
The eight held-out questions cover Portuguese queries over English memories,
literal identifiers and decisions with a closely related, contradictory
answer. Only the historical or current decision requested by the question
counts as relevant. Held-out judgments are fixed before the sweep.
[Calibration fixture](../tests/memory/fixture.json) · [Held-out fixture](../tests/memory/held-out.json)

The verified Blink release produced one winning cutoff, **0.15**. The
production Go ranker over a disposable LadybugDB found the expected source
in the top three on 6/7 original questions, with 3 irrelevant results out of 15 returned.
Holding that cutoff fixed, it found 8/8 expected sources on the held-out
fixture, with 9 irrelevant results out of 17 returned. The GloVe CLI
baseline found 4/8 held-out sources, with 6 irrelevant results out of 10.
Blink retrieved four additional expected sources and three additional
irrelevant results on that fixture.

| Fixture | Blink hits at 3 | Blink irrelevant / returned | GloVe hits at 3 | GloVe irrelevant / returned |
| --- | ---: | ---: | ---: | ---: |
| Original | 6/7 | 3/15 | 5/7 | 10/21 |
| Held-out | 8/8 | 9/17 | 4/8 | 6/10 |

The Go encoder matched token IDs and all 256 coordinates across 14 cases
on the verified release against the pinned Model2Vec reference, with an
absolute coordinate tolerance of `1e-6`.

### Native CLI measurement

The evaluation starts each model with a new temporary database and records
the fixture through the binary. The first fresh-cache query starts a stopped
daemon. The restart query starts it again with populated vectors on disk.
The single-write query runs on a warm daemon immediately after recording one
project memory. Latency is client wall time including process startup, Unix
IPC, search and JSON decoding. Each warm p95 uses five repetitions of every
query with global included. RSS is the whole daemon's resident set from
`ps -o rss=`, sampled just after the first query in each state. These numbers
depend on host and dataset size.

On Linux amd64 (AMD EPYC 9V74, Go 1.27.4, LadybugDB 0.17.0), the GloVe
baseline returned 5/7 expected sources and 10 irrelevant results out of 21
on the calibration fixture. On the held-out fixture it returned 4/8 expected
sources and 6 irrelevant results out of 10. It found both identifiers and two
of the three conflict queries. It missed the three Portuguese-only prompts
and the Portuguese conflict query.

| GloVe data | Cache state | First query (ms) | Warm p95 (ms) | Daemon RSS (MiB) |
| --- | --- | ---: | ---: | ---: |
| Original | Fresh | 3,082.1 | 44.1 | 272.4 |
| Original | Restart | 2,597.9 | 43.8 | 279.7 |
| Original | One write | 37.5 | 43.6 | 323.8 |
| Held-out | Fresh | 2,632.4 | 40.4 | 277.5 |
| Held-out | Restart | 2,529.5 | 37.8 | 270.5 |
| Held-out | One write | 42.7 | 37.9 | 293.8 |

The following Blink measurements used the compiled CLI with the verified
release on a separate GitHub-hosted Ubuntu 24.04 amd64 runner with Go 1.26.11.
The runner verified that the inference processes could use Unix IPC and had
no external IPv4 or IPv6 route. Its CLI reproduced the source hits and
irrelevant counts from the store evaluation on both fixtures.
[Linux validation log and artifacts](https://github.com/ewertonls/bigbrain/actions/runs/46123716970/job/140913570695)

| Blink data | Cache state | First query (ms) | Warm p95 (ms) | Daemon RSS (MiB) |
| --- | --- | ---: | ---: | ---: |
| Original | Fresh | 243.8 | 22.6 | 178.9 |
| Original | Restart | 239.9 | 24.7 | 179.4 |
| Original | One write | 18.7 | 25.0 | 197.7 |
| Held-out | Fresh | 442.1 | 30.2 | 178.1 |
| Held-out | Restart | 227.1 | 39.1 | 176.1 |
| Held-out | One write | 13.0 | 31.9 | 204.8 |

### Reproduction

Run from the checkout root. `make evaluate-memory` exercises both fixtures
and reports per-question source hits, returned keys, irrelevant counts,
timings and process RSS. The store tests count actual document encodes on
restart, scope changes and interrupted rebuilds. The CLI check uses a real
terminal for stderr and parses stdout independently.

The release workflow fetches and verifies the pinned bundle during setup.
The Linux amd64 job runs inference in an empty network namespace and checks
that IPv4 and IPv6 have no external route while Unix IPC works. The macOS
ARM64 job runs the relocated package outside the checkout and verifies the
loaded LadybugDB dylib. Its runner retains ordinary network access. Both jobs
generate official reference vectors and save per-query results, timings and
RSS for both fixtures as JSON artifacts.
[Release validation workflow](../.github/workflows/blink-release.yml)

```sh
./scripts/download-blink.sh build/models/blink
export BIGBRAIN_BLINK_BUNDLE="$PWD/build/models/blink"
export BIGBRAIN_EMBEDDINGS="$BIGBRAIN_BLINK_BUNDLE"
```

The golden generator runs only during development. It uses the official
Model2Vec Python API and its pinned tokenizer/backend to produce token IDs
and all 256 float32 coordinates. The Go test compares absolute coordinate
errors to `1e-6`. A small deterministic table exercises the same code path
in the standard suite. Generate the release golden file from the verified
local bundle before running the release tests.

```sh
python3 -m venv /tmp/bigbrain-reference
/tmp/bigbrain-reference/bin/pip install model2vec==0.11.0 model2vec-rs==0.5.1 tokenizers==0.23.3 numpy==2.4.4
/tmp/bigbrain-reference/bin/python tests/memory/blink_reference.py "$BIGBRAIN_BLINK_BUNDLE"
# To regenerate the small deterministic fixture, omit the bundle argument.
```

Run the cutoff sweep from the checkout root with the same native
library flags as the Makefile.

```sh
make build
CGO_CFLAGS="-I$PWD/lib-ladybug" CGO_LDFLAGS="-L$PWD/lib-ladybug" \
  go test -tags system_ladybug -ldflags "-r '$PWD/lib-ladybug'" \
  -run '^TestBlinkCutoffCalibration$' -v ./internal/store
```

Use the selected cutoff for both fixtures. Run the CLI check with
network access disabled after downloading the bundles.

```sh
make test
make evaluate-memory
python3 tests/memory/cli_blink.py ./build/bigbrain "$BIGBRAIN_BLINK_BUNDLE"
BIGBRAIN_EMBEDDINGS="$PWD/build/models/glove-50.gz" make evaluate-memory
```

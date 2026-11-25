# Memory retrieval evaluation

The fixed questions and relevance judgments come from the LadybugDB spike,
Foundation acceptance and CLI development sessions. The fixture records the
search phrase used for each resumption question and the source of each memory.
Three phrases have no word in common with their target text. Score each query
at three results. A hit requires both the expected ID and its original source.
Count each returned ID outside the question's relevance set as irrelevant.
[Evaluation fixture](../tests/memory/fixture.json)

## Results on Linux amd64

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
A memory write evicts its scope's derived index after commit. The next search builds
it from the database. Index memory grows with the scopes searched, and vector
search scans and sorts the visible scores.

On Linux amd64 with the 50-dimensional GloVe Wiki/Gigaword model, building
text postings for the 14 visible fixture memories took 31.6 µs. Generating
their normalized embeddings took 16.1 µs. Building both together took
47.6 µs and allocated 52,208 bytes per iteration. These in-process Go
benchmarks exclude loading the vocabulary, reading LadybugDB and CLI startup.

## Reproduction

Configure the pinned GloVe model as described in the README, then run the
native evaluation from the repository root. It creates a disposable database,
prints per-question results and timing as JSON, and runs the index benchmarks.
[Model setup](../README.md#offline-hybrid-search)

```sh
export BIGBRAIN_EMBEDDINGS="$PWD/build/models/glove-50.gz"
make evaluate-memory
```

The ranker uses BM25 with k1 of 1.2 and b of 0.75. A direct match contributes
`BM25 / (1 + BM25)`. Cosines of at least 0.45 contribute `0.65 × cosine`.
Each of the top `max(limit, 5)` seeds can contribute up to five visible
neighbors. A target receives its strongest graph contribution,
`0.8 × seed score × weight / (1 + weight)`, once. The experiment used
the default unit link weight and the original GloVe vocabulary throughout.

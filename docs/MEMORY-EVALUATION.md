# Memory retrieval evaluation

The fixed questions and relevance judgments come from the LadybugDB spike,
Foundation acceptance and CLI development sessions. The fixture records the
search phrase used for each resumption question and the source of each memory.
Three phrases have no word in common with their target text. Score each query
at three results. A hit requires both the expected ID and its original source.
Count each returned ID outside the question's relevance set as irrelevant.
[Evaluation fixture](../tests/memory/fixture.json)

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
A write evicts its scope's derived index after commit. The next search builds
it from the database. Index memory grows with the scopes searched, and vector
search scans and sorts the visible scores.

On Linux amd64 with the 50-dimensional GloVe Wiki/Gigaword model, building
text postings for the 14 visible fixture memories took 27.6 µs. Generating
their normalized embeddings took 19.4 µs. Building both together took
60.7 µs and allocated 52,192 bytes per iteration. These in-process Go
benchmarks exclude loading the vocabulary, reading LadybugDB and CLI startup.

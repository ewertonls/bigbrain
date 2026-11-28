package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/ewertonls/bigbrain/internal/protocol"
	bolt "go.etcd.io/bbolt"
)

// Count document calls separately from the search phrase, including documents
// that produce no vector. The counter wraps the encoder used by real queries.
func countDocumentEncodes(t *testing.T, s *Store) *int {
	t.Helper()
	model, reason := s.vectorModel()
	if model == nil {
		t.Fatalf("load model: %s", reason)
	}
	calls := 0
	encode := model.encode
	model.encode = func(text string) []float32 {
		if text != "lookup" {
			calls++
		}
		return encode(text)
	}
	return &calls
}

func TestVectorCacheAcrossScopesAndRestarts(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "vectors.txt")
	if err := os.WriteFile(file, []byte("3 2\nlookup 1 0\ncar 1 0\ntruck 1 0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BIGBRAIN_EMBEDDINGS", file)
	t.Setenv("BIGBRAIN_VECTOR_INDEX", "")
	path := filepath.Join(dir, "memory.lbug")
	s := openTestStore(t, path)
	record := func(scope, key, text string) string {
		return recordForTest(t, s, protocol.RecordRequest{Scope: scope, Key: key, Text: text, Source: key}).ID
	}
	query := func(scope string, events *[]protocol.IndexProgress) protocol.QueryResult {
		var report func(protocol.IndexProgress)
		if events != nil {
			report = func(p protocol.IndexProgress) { *events = append(*events, p) }
		}
		result, err := s.QueryWithProgress(context.Background(), protocol.QueryRequest{Scope: scope, Text: "lookup", Mode: "hybrid", IncludeGlobal: true}, report)
		if err != nil || result.Fallback != "" {
			t.Fatalf("query: %+v, %v", result, err)
		}
		return result
	}
	a := record("project:A", "a", "car")
	record("project:B", "b", "truck")
	g := record("global", "g", "truck car")
	record("global", "empty", "not-in-vocabulary")
	calls := countDocumentEncodes(t, s)
	query("project:A", nil)
	query("project:B", nil)
	if *calls != 4 {
		t.Fatalf("first A+B with global = %d encodes, want 4", *calls)
	}
	var events []protocol.IndexProgress
	query("project:A", &events)
	if len(s.indexes) != 2 || s.indexes["project:B"] != nil || len(events) != 0 || *calls != 4 {
		t.Fatalf("scope eviction/global reuse: %d, %v", *calls, events)
	}
	s.Close()
	s = openTestStore(t, path)
	calls = countDocumentEncodes(t, s)
	query("project:A", &events)
	query("project:B", &events)
	if *calls != 0 || len(events) != 0 {
		t.Fatalf("populated restart: %d, %v", *calls, events)
	}
	id := record("project:B", "new", "car truck")
	query("project:B", &events)
	want := []protocol.IndexProgress{{Phase: "recalculation", Scope: "project:B", Total: 1}, {Phase: "recalculation", Scope: "project:B", Completed: 1, Total: 1}}
	if *calls != 1 || !reflect.DeepEqual(events, want) {
		t.Fatalf("single write: %d, %v", *calls, events)
	}
	if _, err := s.Invalidate(protocol.InvalidateRequest{MemoryWriteRequest: protocol.MemoryWriteRequest{Scope: "project:A", Key: "inv", ID: a}, Reason: "old", Source: "test"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Delete(protocol.MemoryWriteRequest{Scope: "global", Key: "del", ID: g}); err != nil {
		t.Fatal(err)
	}
	checkHidden := func() {
		for _, scope := range []string{"project:A", "project:B"} {
			q := protocol.QueryRequest{Scope: scope, Text: "lookup", Mode: "hybrid", IncludeGlobal: true}
			result := queryForTest(t, s, q)
			for _, m := range result.Matches {
				if m.ID == a || m.ID == g {
					t.Fatalf("revoked memory in %+v", result)
				}
			}
			if scope == "project:B" && len(result.Matches) != 2 {
				t.Fatalf("missing active memory %s: %+v", id, result)
			}
		}
	}
	checkHidden()
	s.Close()
	s = openTestStore(t, path)
	defer s.Close()
	calls = countDocumentEncodes(t, s)
	checkHidden()
	if *calls != 0 {
		t.Fatalf("revocations required %d encodes after restart", *calls)
	}
}

func TestVectorRebuildResumesCommittedWork(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "vectors.txt")
	configure := func(n int) {
		err := os.WriteFile(model, []byte(fmt.Sprintf("2 2\nlookup 1 %d\ncar 1 %d\n", n, n)), 0600)
		if err != nil {
			t.Fatal(err)
		}
	}
	configure(0)
	t.Setenv("BIGBRAIN_EMBEDDINGS", model)
	t.Setenv("BIGBRAIN_VECTOR_INDEX", "")
	path := filepath.Join(dir, "memory.lbug")
	s := openTestStore(t, path)
	const n = 5
	q := protocol.QueryRequest{Scope: "global", Text: "lookup", Mode: "hybrid"}
	for i := range n {
		recordForTest(t, s, protocol.RecordRequest{Scope: "global", Key: fmt.Sprint(i), Text: "car", Source: "test"})
	}
	queryForTest(t, s, q)
	info, err := os.Stat(model)
	if err != nil {
		t.Fatal(err)
	}
	// An in-place model update can preserve the inode, size and mtime.
	configure(1)
	if err := os.Chtimes(model, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	calls := countDocumentEncodes(t, s)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var got []protocol.IndexProgress
	_, err = s.QueryWithProgress(ctx, q, func(p protocol.IndexProgress) {
		got = append(got, p)
		if p.Phase == "recalculation" && p.Completed == 2 {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) || *calls != 2 {
		t.Fatalf("interrupted: err=%v encodes=%d", err, *calls)
	}
	var want []protocol.IndexProgress
	phase := func(name string, total, done int) {
		for i := 0; i <= done; i++ {
			want = append(want, protocol.IndexProgress{Phase: name, Scope: "global", Total: total, Completed: i})
		}
	}
	phase("invalidation", n, n)
	phase("recalculation", n, 2)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("interrupted progress: %+v", got)
	}
	s.Close()
	s = openTestStore(t, path)
	defer s.Close()
	calls = countDocumentEncodes(t, s)
	got, want = nil, nil
	result, err := s.QueryWithProgress(context.Background(), q, func(p protocol.IndexProgress) { got = append(got, p) })
	phase("recalculation", n-2, n-2)
	if err != nil || len(result.Matches) != n || *calls != n-2 || !reflect.DeepEqual(got, want) {
		t.Fatalf("resumed: %+v, err=%v encodes=%d progress=%v", result, err, *calls, got)
	}
	// Simulate restoring a record with the same ID but different text.
	_, err = s.query("MATCH (m:Memory {id:$id}) SET m.text='changed car';", map[string]any{"id": result.Matches[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	clear(s.indexes)
	queryForTest(t, s, q)
	if *calls != n-1 {
		t.Fatalf("changed text: wanted one additional encode; total=%d", *calls)
	}
}

func TestDamagedVectorEntryAndUnavailableCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store.lbug")
	model := filepath.Join(dir, "words")
	if err := os.WriteFile(model, []byte("2 1\nlookup 1\ncar 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BIGBRAIN_EMBEDDINGS", model)
	t.Setenv("BIGBRAIN_VECTOR_INDEX", "")
	s := openTestStore(t, path)
	id := recordForTest(t, s, protocol.RecordRequest{Scope: "global", Key: "k", Text: "car", Source: "document"}).ID
	q := protocol.QueryRequest{Scope: "global", Text: "lookup", Mode: "hybrid"}
	queryForTest(t, s, q)
	err := s.cache.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("global"))
		key := []byte(id)
		damaged := slices.Clone(bucket.Get(key))
		damaged[len(damaged)-1] ^= 1
		return bucket.Put(key, damaged)
	})
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	s = openTestStore(t, path)
	calls := countDocumentEncodes(t, s)
	if m := queryForTest(t, s, q); *calls != 1 || len(m.Matches) != 1 || m.Matches[0].ID != id {
		t.Fatalf("corrupt entry: %v %d", m, *calls)
	}
	s.Close()
	if err := os.Rename(path+".vectors", path+".saved-vectors"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path+".vectors", 0700); err != nil {
		t.Fatal(err)
	}
	s = openTestStore(t, path)
	defer s.Close()
	q.Text = "car"
	result := queryForTest(t, s, q)
	if result.Fallback != "vector_cache_unavailable" || len(result.Matches) != 1 || result.Matches[0].ID != id {
		t.Fatalf("cache fallback: %+v", result)
	}
}

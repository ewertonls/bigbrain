package store

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

func TestVisibleBM25AndExactNeighbors(t *testing.T) {
	db := openTestStore(t, filepath.Join(t.TempDir(), "store.lbug"))
	record := func(scope, key, text string) string {
		return recordForTest(t, db, protocol.RecordRequest{Scope: scope, Key: key, Text: text, Source: key}).ID
	}
	// The x-coordinate of each normalized vector is its exact cosine to x.
	model := &embeddings{dim: 2, words: map[string][]float32{"x": {1, 0}, "near": {3, 4}, "far": {-1, 0}}}
	near := record("project:B", "near", "near near")
	far := record("project:B", "far", "far")
	global := record("global", "global", "x")
	check := func(include bool) []protocol.Match {
		visible, err := db.visibleIndexes(protocol.QueryRequest{Scope: "project:B", IncludeGlobal: include})
		if err != nil {
			t.Fatal(err)
		}
		visible.indexVectors(model)
		got := visible.nearest(model.encode("x"), 2)
		expected := []string{near, far}
		cosines := []float64{0.6, -1}
		n, avg := 2., 1.5
		if include {
			expected = []string{global, near}
			cosines = []float64{1, 0.6}
			n, avg = 3, 4./3
		}
		for i, id := range expected {
			if len(got) != 2 || got[i].ID != id || math.Abs(got[i].Reason.Cosine-cosines[i]) > 1e-6 {
				t.Fatalf("exact neighbors %v: %+v", expected, got)
			}
		}
		lex := visible.textMatches("near near")
		want := math.Log(1+(n-1+0.5)/1.5) * 2 * 2.2 / (2 + 1.2*(0.25+0.75*2/avg))
		if len(lex) != 1 || lex[0].ID != near || math.Abs(lex[0].Reason.BM25-want) > 1e-10 {
			t.Fatalf("BM25 = %+v, want %f", lex, want)
		}
		return append(got, lex...)
	}
	b, bg := check(false), check(true)
	for i := range 12 {
		record("project:A", fmt.Sprintf("duplicate%d", i), "x near near")
	}
	// Force cold builds as well, so isolation cannot pass because of a warm cache.
	clear(db.indexes)
	if !reflect.DeepEqual(b, check(false)) || !reflect.DeepEqual(bg, check(true)) {
		t.Fatal("another project's terms or vectors changed visible ranking")
	}
}

func TestNearestBreaksTiesAcrossScopes(t *testing.T) {
	indexes := collection{
		buildIndex([]protocol.Memory{{ID: "z", Text: "car"}}),
		buildIndex([]protocol.Memory{{ID: "a", Text: "car"}}),
	}
	model := &embeddings{dim: 1, words: map[string][]float32{"car": {1}}}
	indexes.indexVectors(model)
	got := indexes.nearest(model.encode("car"), 1)
	if len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("nearest = %+v, want a", got)
	}
}

func TestLoadEmbeddingVocabulary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vectors.txt")
	for _, data := range []string{"2 2\nword 0 1\n", "1 2\nword NaN 1\n", "2 1\nword 1\nword 2\n"} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadEmbeddings(path); err == nil {
			t.Fatalf("accepted malformed vectors %q", data)
		}
	}
	if err := os.WriteFile(path, []byte("5 2\ncar 1 0\nvehicle 1 0\nLondon 0 1\nUS -1 0\nus 0 -1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	model, err := loadEmbeddings(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(model.encode("the vehicle"), model.encode("car")) || model.encode("absent") != nil {
		t.Fatal("unexpected sentence embeddings")
	}
	for word, want := range map[string][]float32{"London": {0, 1}, "US": {-1, 0}, "us": {0, -1}, "Car": {1, 0}} {
		if got := model.encode(word); !reflect.DeepEqual(got, want) {
			t.Errorf("encode(%q) = %v, want %v", word, got, want)
		}
	}
}

func BenchmarkMemoryIndex(b *testing.B) {
	data, err := os.ReadFile("../../tests/memory/fixture.json")
	if err != nil {
		b.Fatal(err)
	}
	var fixture struct {
		Memories []struct{ Key, Scope, Text, Source string }
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		b.Fatal(err)
	}
	var memories []protocol.Memory
	for _, m := range fixture.Memories {
		if m.Scope == "bigbrain" || m.Scope == "global" {
			memories = append(memories, protocol.Memory{ID: m.Key, Scope: m.Scope, Text: m.Text, Source: m.Source})
		}
	}
	b.Run("text", func(b *testing.B) {
		for b.Loop() {
			buildIndex(memories)
		}
	})
	path := os.Getenv("BIGBRAIN_EMBEDDINGS")
	if path == "" {
		return
	}
	model, err := loadEmbeddings(path)
	if err != nil {
		b.Fatal(err)
	}
	b.Run("embeddings", func(b *testing.B) {
		for b.Loop() {
			for _, m := range memories {
				model.encode(m.Text)
			}
		}
	})
	b.Run("text-and-vectors", func(b *testing.B) {
		for b.Loop() {
			collection{buildIndex(memories)}.indexVectors(model)
		}
	})
}

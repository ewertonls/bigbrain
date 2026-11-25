package store

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

func TestHybridExplainsSemanticAndGraphResults(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "model.txt")
	if err := os.WriteFile(model, []byte("3 2\ncar 1 0\nautomobile 1 0\nforest 0 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BIGBRAIN_EMBEDDINGS", model)
	t.Setenv("BIGBRAIN_VECTOR_INDEX", "")
	path := filepath.Join(dir, "db.lbug")
	db := openTestStore(t, path)
	seed := recordForTest(t, db, protocol.RecordRequest{Key: "d", Scope: "project:A", Text: "car", Source: "Decision"})
	evidence := recordForTest(t, db, protocol.RecordRequest{Key: "e", Scope: "global", Text: "evidence", Source: "Trial"})
	link := linkForTest(t, db, protocol.LinkRequest{Key: "l", Scope: "project:A", FromID: evidence.ID, ToID: seed.ID, Kind: "supports", Source: "Review", Weight: 2})
	q := protocol.QueryRequest{Scope: "project:A", Text: "automobile", Mode: "hybrid", IncludeGlobal: true}
	got := queryForTest(t, db, q)
	if got.Fallback != "" || got.Count != 2 || got.Matches[0].ID != seed.ID || got.Matches[0].Reason.Kind != "vector" || got.Matches[0].Reason.Cosine != 1 || got.Matches[0].Source != "Decision" {
		t.Fatalf("semantic match: %+v", got)
	}
	n := got.Matches[1]
	if n.ID != evidence.ID || n.Source != "Trial" || n.Reason.GraphScore <= 0 || n.Reason.Path == nil || n.Reason.Path.Direction != "reverse" || n.Reason.Path.RelationshipID != link.ID || n.Reason.Path.Source != "Review" {
		t.Fatalf("graph match: %+v", n)
	}
	q.IncludeGlobal = false
	if private := queryForTest(t, db, q); private.Count != 1 || private.Matches[0].Reason.Path != nil {
		t.Fatalf("scope leak: %+v", private)
	}
	noVocabulary := protocol.QueryRequest{Scope: "project:A", Text: "nonvocabulary", Mode: "hybrid"}
	if got := queryForTest(t, db, noVocabulary); got.Fallback != "no_query_embedding" {
		t.Fatalf("query coverage: %+v", got)
	}
	if got := queryForTest(t, db, protocol.QueryRequest{Scope: "global", Text: "automobile", Mode: "hybrid"}); got.Fallback != "no_visible_embeddings" {
		t.Fatalf("scope coverage: %+v", got)
	}
	q.Text = "car"
	q.IncludeGlobal = true
	q.Mode = "associative"
	baseline := queryForTest(t, db, q)
	db.Close()
	for _, test := range []struct{ index, path, want string }{
		{"off", model, "vector_index_disabled"}, {"", model + ".missing", "model_unavailable"}, {"", "", "model_not_configured"},
	} {
		t.Setenv("BIGBRAIN_EMBEDDINGS", test.path)
		t.Setenv("BIGBRAIN_VECTOR_INDEX", test.index)
		db = openTestStore(t, path)
		q.Mode = "hybrid"
		actual := queryForTest(t, db, q)
		if actual.Fallback != test.want || !reflect.DeepEqual(actual.Matches, baseline.Matches) || actual.Count != baseline.Count {
			t.Fatalf("fallback %s: %+v", test.want, actual)
		}
		db.Close()
	}
}

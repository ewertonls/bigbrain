package store

import (
	"path/filepath"
	"testing"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

func TestBrowseScopeAndPagination(t *testing.T) {
	db := openTestStore(t, filepath.Join(t.TempDir(), "memory.lbug"))
	add := func(key, scope string) string {
		return recordForTest(t, db, protocol.RecordRequest{Scope: scope, Key: key, Text: key, Source: "fixture"}).ID
	}
	first := add("first", "project:A")
	second := add("second", "project:A")
	hidden := add("private", "project:B")
	shared := add("shared", "global")
	q := protocol.BrowseRequest{Scope: "project:A", Limit: 1}
	page := func() protocol.BrowseResult {
		r, err := db.Browse(q)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	r := page()
	if len(r.Memories) != 1 || r.Memories[0].ID != second || r.NextCursor != second {
		t.Fatal(r)
	}
	q.After = r.NextCursor
	r = page()
	if len(r.Memories) != 1 || r.Memories[0].ID != first || r.NextCursor != "" {
		t.Fatal(r)
	}
	q.After = hidden
	_, err := db.Browse(q)
	requireCode(t, err, "INVALID_CURSOR")
	q.After = ""
	q.IncludeGlobal = true
	r = page()
	if r.Memories[0].ID != shared {
		t.Fatal(r)
	}
	q.Scope = "global"
	_, err = db.Browse(q)
	if err == nil {
		t.Fatal("global union accepted")
	}
}

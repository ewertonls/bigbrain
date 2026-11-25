package store

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	lbug "github.com/LadybugDB/go-ladybug"
	"github.com/ewertonls/bigbrain/internal/protocol"
)

func TestMemoryLifecycleSurvivesRestartAndImport(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "model.txt")
	if err := os.WriteFile(model, []byte("3 2\nreceiver 1 0\nreceiverunit 1 0\norbit 0 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BIGBRAIN_EMBEDDINGS", model)
	t.Setenv("BIGBRAIN_VECTOR_INDEX", "")
	path := filepath.Join(dir, "original.lbug")
	db := openTestStore(t, path)
	record := func(key, scope, text, kind string) protocol.RecordRequest {
		return protocol.RecordRequest{Key: key, Scope: scope, Text: text, Type: kind, Source: "Source " + key}
	}
	d := recordForTest(t, db, record("d", "project:A", "Orbit", "decision"))
	eRequest := record("e", "project:A", "receiver", "observation")
	e := recordForTest(t, db, eRequest)
	g := recordForTest(t, db, record("g", "global", "Published lesson", "procedural"))
	b := recordForTest(t, db, record("b", "project:B", "Private experiment", "observation"))
	link := func(key, scope, from, to string) protocol.LinkRequest {
		return protocol.LinkRequest{Key: key, Scope: scope, FromID: from, ToID: to, Kind: "cites", Source: "Review " + key, Weight: 1}
	}
	dToE := link("de", "project:A", d.ID, e.ID)
	eToG := link("eg", "project:A", e.ID, g.ID)
	bToG := link("bg", "project:B", b.ID, g.ID)
	linkForTest(t, db, dToE)
	linkForTest(t, db, eToG)
	linkForTest(t, db, bToG)
	q := protocol.QueryRequest{Scope: "project:A", Text: "receiverunit", Mode: "hybrid"}
	if got := queryForTest(t, db, q); got.Matches[0].ID != e.ID {
		t.Fatalf("missing initial embedding: %+v", got)
	}

	owned := protocol.MemoryWriteRequest{Scope: "project:A", ID: e.ID, Key: "invalidate"}
	inv := protocol.InvalidateRequest{MemoryWriteRequest: owned, Reason: "Trial superseded", Source: "Review 2"}
	if got, err := db.Invalidate(inv); err != nil || got.ID != e.ID || got.Replayed {
		t.Fatalf("invalidate: %+v, %v", got, err)
	}
	if got, err := db.Invalidate(inv); err != nil || !got.Replayed {
		t.Fatalf("invalidate retry: %+v, %v", got, err)
	}
	for _, query := range []protocol.QueryRequest{q, {Scope: "project:A", ID: e.ID}, {Scope: "project:A", Text: "receiver", Mode: "text"}} {
		if got := queryForTest(t, db, query); got.Count != 0 {
			t.Fatalf("invalid memory indexed: %+v", got)
		}
	}
	if got := queryForTest(t, db, protocol.QueryRequest{Scope: "project:A", Text: "Orbit", Mode: "associative"}); got.Count != 1 || got.Matches[0].ID != d.ID {
		t.Fatalf("invalid relationship in query: %+v", got)
	}
	newLink := dToE
	newLink.Key = "new link after invalidation"
	_, err := db.Link(newLink)
	requireCode(t, err, "NOT_FOUND")
	h := func(scope, id string, global bool) protocol.HistoryResult {
		t.Helper()
		actual, err := db.History(protocol.HistoryRequest{Scope: scope, ID: id, IncludeGlobal: global})
		if err != nil {
			t.Fatal(err)
		}
		return actual
	}
	history := h("project:A", e.ID, false)
	if history.Status != "invalidated" || history.Invalidation == nil || history.Invalidation.Reason != inv.Reason ||
		history.Invalidation.Source != inv.Source || history.Invalidation.At == "" || len(history.Related) != 1 ||
		history.Related[0].ID != d.ID || history.Related[0].Type != "decision" || history.Related[0].Status != "active" ||
		history.Related[0].Path.Direction != "reverse" || history.Related[0].Path.Source != dToE.Source {
		t.Fatalf("invalidation history: %+v", history)
	}
	if got := h("project:A", d.ID, false); got.Status != "active" || got.Related[0].Status != "invalidated" {
		t.Fatalf("decision changed: %+v", got)
	}
	if got := h("global", g.ID, false); len(got.Related) != 0 {
		t.Fatalf("global history leaked project links: %+v", got)
	}
	page := protocol.HistoryRequest{Scope: "project:A", ID: e.ID, IncludeGlobal: true, Limit: 1}
	first, err := db.History(page)
	if err != nil || len(first.Related) != 1 || first.NextCursor == "" {
		t.Fatalf("first page: %+v, %v", first, err)
	}
	page.After = first.NextCursor
	second, err := db.History(page)
	if err != nil || len(second.Related) != 1 || second.NextCursor != "" || first.Related[0].ID == second.Related[0].ID {
		t.Fatalf("second page: %+v, %v", second, err)
	}
	db.Close()
	db = openTestStore(t, path)
	if got := h("project:A", e.ID, false); !reflect.DeepEqual(got, history) {
		t.Fatalf("history changed on reopen: %+v", got)
	}
	remove := protocol.MemoryWriteRequest{Key: "delete", Scope: "project:A", ID: e.ID}
	wrongScope := remove
	wrongScope.Scope = "project:B"
	_, err = db.Delete(wrongScope)
	requireCode(t, err, "NOT_FOUND")
	if got, err := db.Delete(remove); err != nil || got.ID != e.ID || got.Replayed {
		t.Fatalf("delete: %+v, %v", got, err)
	}
	checkDeletion := func() {
		t.Helper()
		_, err := db.Record(eRequest)
		requireCode(t, err, "RESOURCE_DELETED")
		for _, request := range []protocol.LinkRequest{dToE, eToG} {
			_, err = db.Link(request)
			requireCode(t, err, "RESOURCE_DELETED")
		}
		changed := eRequest
		changed.Text = "Different payload"
		_, err = db.Record(changed)
		requireCode(t, err, "IDEMPOTENCY_CONFLICT")
		changed = eRequest
		changed.Scope = "project:B"
		_, err = db.Record(changed)
		requireCode(t, err, "IDEMPOTENCY_CONFLICT")
		wrongOperation := remove
		wrongOperation.Key = eRequest.Key
		_, err = db.Delete(wrongOperation)
		requireCode(t, err, "IDEMPOTENCY_CONFLICT")
		if replay, err := db.Delete(remove); err != nil || !replay.Replayed || replay.ID != e.ID {
			t.Fatalf("deletion replay: %+v, %v", replay, err)
		}
		_, err = db.History(protocol.HistoryRequest{Scope: "project:A", ID: e.ID})
		requireCode(t, err, "NOT_FOUND")
		clear(db.indexes)
		if got := queryForTest(t, db, q); got.Count != 0 {
			t.Fatalf("rebuild restored deleted embedding: %+v", got)
		}
		if got := h("project:A", g.ID, true); got.Memory.ID != g.ID || len(got.Related) != 0 {
			t.Fatalf("published lesson or private link: %+v", got)
		}
		if got := h("project:B", g.ID, true); len(got.Related) != 1 || got.Related[0].ID != b.ID {
			t.Fatalf("unrelated project lost its link: %+v", got)
		}
		rows, err := db.query("MATCH (r:Receipt) WHERE r.deleted=true RETURN r.key,r.operation,r.scope,r.result_id,r.fingerprint;", nil)
		if err != nil || len(rows) != 3 {
			t.Fatalf("tombstones: %v, %v", rows, err)
		}
		for _, row := range rows {
			if row[1] != nil || row[2] != nil || row[3] != nil || len(row[4].(string)) != 64 {
				t.Fatalf("tombstone retained resource data: %v", row)
			}
		}
	}
	checkDeletion()
	db.Close()
	db = openTestStore(t, path)
	checkDeletion()
	archive := filepath.Join(dir, "export")
	quoted := "'" + strings.ReplaceAll(archive, "'", "''") + "'"
	if _, err = db.query("EXPORT DATABASE "+quoted+" (FORMAT='parquet');", nil); err != nil {
		t.Fatal(err)
	}
	db.Close()
	restored := filepath.Join(dir, "restored.lbug")
	raw, err := lbug.OpenDatabase(restored, lbug.DefaultSystemConfig())
	if err != nil {
		t.Fatal(err)
	}
	conn, err := lbug.OpenConnection(raw)
	if err != nil {
		t.Fatal(err)
	}
	result, err := conn.Query("IMPORT DATABASE " + quoted + ";")
	if err != nil {
		t.Fatal(err)
	}
	result.Close()
	conn.Close()
	raw.Close()
	db = openTestStore(t, restored)
	checkDeletion()
	_, err = db.Delete(protocol.MemoryWriteRequest{Key: "remove-global", Scope: "global", ID: g.ID})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Link(bToG)
	requireCode(t, err, "RESOURCE_DELETED")
	if got := h("project:B", b.ID, true); got.Status != "active" || len(got.Related) != 0 {
		t.Fatalf("global deletion changed the project memory: %+v", got)
	}

}

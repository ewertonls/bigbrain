package store

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

func TestReconcilePublishedLessonWithPrivateCounterexample(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.lbug")
	db := openTestStore(t, path)
	record := func(scope, text, source string) string {
		return recordForTest(t, db, protocol.RecordRequest{Scope: scope, Key: text, Text: text, Source: source}).ID
	}
	observation := record("project:A", "Receiver failed at peak load.", "A's private trial")
	proposal := consolidateForTest(t, db, protocol.ConsolidateRequest{Scope: "project:A", Key: "draft", EvidenceIDs: []string{observation}})
	approved := protocol.PublishRequest{Scope: "project:A", Key: "publish", ProposalID: proposal.ID, Approve: true,
		Text: "Batch receiver requests.", Source: "Approved quota guidance"}
	lesson, err := db.Publish(approved)
	if err != nil {
		t.Fatal(err)
	}
	const scope = "project:B"
	counter := record(scope, "Batching increased latency on dedicated receivers.", "B trial 7")
	request := protocol.ReconcileRequest{Scope: scope, Key: "review", SupportingID: lesson.ID, ContraryID: counter,
		Outcome: "supersedes", SupersedingSide: "contrary", Text: "Use immediate delivery for dedicated receivers.",
		AppliesWhen: "Dedicated receivers have no shared quota.", Rationale: "B's trial measured excess batch latency.",
		Author: "B reviewer", Source: "Review 12"}
	_, err = db.Reconcile(request)
	requireCode(t, err, "NOT_FOUND")
	request.IncludeGlobal = true
	invisible := request
	invisible.SupportingID = observation
	_, err = db.Reconcile(invisible)
	requireCode(t, err, "NOT_FOUND")
	review, err := db.Reconcile(request)
	if err != nil || review.ID == "" || review.Replayed {
		t.Fatalf("create review: %+v, %v", review, err)
	}
	history := func(id, scope string, global bool) protocol.HistoryResult {
		t.Helper()
		h, err := db.History(protocol.HistoryRequest{Scope: scope, IncludeGlobal: global, ID: id})
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	full := history(review.ID, scope, true)
	want := protocol.Reconciliation{Outcome: request.Outcome, SupersedingSide: request.SupersedingSide,
		AppliesWhen: request.AppliesWhen, Rationale: request.Rationale, Author: request.Author}
	if full.Memory.Type != "reconciliation" || full.Memory.Text != request.Text || full.Memory.Source != request.Source ||
		full.Memory.Scope != scope || full.Reconciliation == nil || *full.Reconciliation != want || len(full.Related) != 2 {
		t.Fatalf("reconciliation: %+v, metadata %+v", full, full.Reconciliation)
	}
	claims := map[string]string{"supporting_claim": lesson.ID, "contrary_claim": counter}
	for _, related := range full.Related {
		p := related.Path
		wantSource := approved.Source
		if p.Kind == "contrary_claim" {
			wantSource = "B trial 7"
		}
		if related.ID != claims[p.Kind] || related.Source != wantSource || p.Scope != scope ||
			p.FromID != review.ID || p.ToID != related.ID || p.Direction != "forward" || p.Source != request.Source {
			t.Fatalf("lost claim or review provenance: %+v", related)
		}
		delete(claims, p.Kind)
	}
	local := history(review.ID, scope, false)
	if len(local.Related) != 1 || local.Related[0].ID != counter {
		t.Fatalf("global claim visible without opt-in: %+v", local)
	}
	for _, reader := range []string{"global", "project:A"} {
		_, err := db.History(protocol.HistoryRequest{Scope: reader, IncludeGlobal: reader != "global", ID: review.ID})
		requireCode(t, err, "NOT_FOUND")
		for _, related := range history(lesson.ID, reader, reader != "global").Related {
			if related.ID == review.ID || related.Path.Scope == scope {
				t.Fatalf("B's review visible from the global lesson in %s: %+v", reader, related)
			}
		}
	}
	// An ordinary relationship cannot relabel a third memory as a reviewed claim.
	_, err = db.Link(protocol.LinkRequest{Scope: scope, Key: "extra", FromID: review.ID, ToID: counter,
		Kind: "supporting_claim", Source: "Later report", Weight: 1})
	requireCode(t, err, "NOT_FOUND")
	db.Close()
	db = openTestStore(t, path)
	if got := history(review.ID, scope, true); !reflect.DeepEqual(got, full) {
		t.Fatalf("review history changed after reopen: %+v", got)
	}
	flipped := request
	flipped.SupersedingSide = "supporting"
	_, err = db.Reconcile(flipped)
	requireCode(t, err, "IDEMPOTENCY_CONFLICT")
	_, err = db.Invalidate(protocol.InvalidateRequest{MemoryWriteRequest: protocol.MemoryWriteRequest{Scope: scope, Key: "withdraw", ID: counter},
		Reason: "Repeating the trial", Source: "Review 13"})
	if err != nil {
		t.Fatal(err)
	}
	fresh := request
	fresh.Key = "new review"
	_, err = db.Reconcile(fresh)
	requireCode(t, err, "NOT_FOUND")
	if got, err := db.Reconcile(request); err != nil || !got.Replayed || got.ID != review.ID {
		t.Fatalf("review replay after trial invalidation: %+v, %v", got, err)
	}
	if _, err := db.Delete(protocol.MemoryWriteRequest{Scope: scope, Key: "delete", ID: review.ID}); err != nil {
		t.Fatal(err)
	}
	_, err = db.Reconcile(request)
	requireCode(t, err, "RESOURCE_DELETED")
}

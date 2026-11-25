package store

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

func TestProjectFeedbackIsolatesGlobalEdges(t *testing.T) {
	db := openTestStore(t, filepath.Join(t.TempDir(), "memory.lbug"))
	g := propagationFixture{t, db}
	seed := g.record("global", "radio")
	low, high := g.record("global", "low"), g.record("global", "high")
	proof := g.record("project:A", "successful trial")
	g.record("project:B", "radio b")
	edge := g.link("global", "weak", seed, low, 1, nil)
	g.link("global", "strong", seed, high, 1.5, nil)
	query := func(scope string, global bool) protocol.QueryResult {
		return queryForTest(t, db, protocol.QueryRequest{Scope: scope, IncludeGlobal: global, Text: "radio", Mode: "associative"})
	}
	bLocal, bWithGlobal, global := query("project:B", false), query("project:B", true), query("global", false)
	r := protocol.FeedbackRequest{FeedbackTarget: protocol.FeedbackTarget{Scope: "project:A", Key: "use", RelationshipID: edge,
		Author: "agent", Source: "Run 7", IncludeGlobal: true}, Confidence: new(1.0), EvidenceID: proof}
	use, err := db.Feedback(r)
	if err != nil {
		t.Fatal(err)
	}
	if replay, err := db.Feedback(r); err != nil || !replay.Replayed || replay.ID != use.ID {
		t.Fatalf("feedback replay: %+v, %v", replay, err)
	}
	a := query("project:A", true)
	if len(a.Matches) != 3 || a.Matches[1].ID != low {
		t.Fatalf("local reinforcement failed to change priority: %+v", a)
	}
	p := a.Matches[1].Reason.Path
	if p.RelationshipID != edge || p.Weight != 2 || p.Confidence == nil || *p.Confidence != 1 ||
		p.ActivationCount == nil || *p.ActivationCount != 1 || p.LastActivatedAt == "" {
		t.Fatalf("private overlay: %+v", p)
	}
	if !reflect.DeepEqual(a, query("project:A", true)) || !reflect.DeepEqual(bLocal, query("project:B", false)) ||
		!reflect.DeepEqual(bWithGlobal, query("project:B", true)) || !reflect.DeepEqual(global, query("global", false)) {
		t.Fatal("repeat read mutated a path or private feedback changed another scope")
	}
	history, err := db.History(protocol.HistoryRequest{Scope: "project:A", IncludeGlobal: true, ID: low})
	if err != nil || len(history.Related) != 1 || history.Related[0].Path.Weight != 2 || history.Related[0].Path.Direction != "reverse" {
		t.Fatalf("reverse history: %+v, %v", history, err)
	}
	q := protocol.FeedbackHistoryRequest{Scope: "project:A", IncludeGlobal: true, RelationshipID: edge}
	h, err := db.FeedbackHistory(q)
	if err != nil || len(h.Events) != 1 || h.Events[0].ID != use.ID || h.Events[0].EvidenceID != proof ||
		h.Events[0].Author != r.Author || h.Events[0].Source != r.Source || h.Events[0].Weight != 2 {
		t.Fatalf("project audit: %+v, %v", h, err)
	}
	q.Scope = "project:B"
	if b, err := db.FeedbackHistory(q); err != nil || len(b.Events) != 0 {
		t.Fatalf("project B audit: %+v, %v", b, err)
	}
	q.After = use.ID
	_, err = db.FeedbackHistory(q)
	requireCode(t, err, "INVALID_REQUEST")
	q.After = ""
	review := protocol.ReviewWeightRequest{FeedbackTarget: protocol.FeedbackTarget{Scope: "global", Key: "review", RelationshipID: edge,
		Author: "reviewer", Source: "approved review"}, Approve: true, Weight: new(3.0)}
	if _, err = db.ReviewWeight(review); err != nil {
		t.Fatal(err)
	}
	if got := matchByID(t, query("global", false), low).Reason.Path; got.Weight != 3 || *got.ActivationCount != 0 || got.Confidence != nil {
		t.Fatalf("global reviewed weight: %+v", got)
	}
	if b, err := db.FeedbackHistory(q); err != nil || len(b.Events) != 1 || b.Events[0].Action != "review-weight" {
		t.Fatalf("global review in B: %+v, %v", b, err)
	}
	if !reflect.DeepEqual(a, query("project:A", true)) {
		t.Fatal("review overwrote the explicit project override")
	}
	privateForget := protocol.ForgetRequest{FeedbackTarget: r.FeedbackTarget, Before: "2999-01-01T00:00:00Z"}
	privateForget.Key = "forget in A"
	if _, err := db.Forget(privateForget); err != nil {
		t.Fatal(err)
	}
	faded := matchByID(t, query("project:A", true), low).Reason.Path
	if faded.Weight != 1 || faded.LastActivatedAt != p.LastActivatedAt || *faded.ActivationCount != 1 ||
		matchByID(t, query("project:B", true), low).Reason.Path.Weight != 3 {
		t.Fatalf("forgetting a project override: %+v", faded)
	}
}

func TestFeedbackForgettingAndAuditLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.lbug")
	db := openTestStore(t, path)
	g := propagationFixture{t, db}
	const scope = "project:A"
	seed, target := g.record(scope, "decision"), g.record(scope, "experiment")
	edge := g.link(scope, "baseline", seed, target, 3, nil)
	proof := g.record("global", "trial 42")
	base := protocol.FeedbackTarget{Scope: scope, Key: "used", RelationshipID: edge, Author: "reviewer", Source: "log"}
	use := protocol.FeedbackRequest{FeedbackTarget: base, EvidenceID: proof, Confidence: new(0.5)}
	_, err := db.Feedback(use)
	requireCode(t, err, "NOT_FOUND")
	use.IncludeGlobal = true
	used, err := db.Feedback(use)
	if err != nil {
		t.Fatal(err)
	}
	forget := protocol.ForgetRequest{FeedbackTarget: base, Before: "2000-01-01T00:00:00Z"}
	forget.Key = "policy"
	_, err = db.Forget(forget)
	requireCode(t, err, "POLICY_NOT_APPLICABLE")
	forget.Before = "2999-01-01T00:00:00Z"
	forgotten, err := db.Forget(forget)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	db = openTestStore(t, path)
	q := protocol.FeedbackHistoryRequest{Scope: scope, RelationshipID: edge}
	h, err := db.FeedbackHistory(q)
	if err != nil || len(h.Events) != 2 {
		t.Fatalf("audit after restart: %+v, %v", h, err)
	}
	for _, ev := range h.Events {
		if ev.EvidenceID != "" || ev.Confidence == nil || *ev.Confidence != 0.5 || ev.ActivationCount == nil || *ev.ActivationCount != 1 {
			t.Fatalf("unexpected metadata or global evidence leaked: %+v", ev)
		}
		if ev.ID == forgotten.ID && (ev.Action != "forget" || ev.Weight != 2 || ev.Cutoff != forget.Before) {
			t.Fatalf("forget audit: %+v", ev)
		}
	}
	got := queryForTest(t, db, protocol.QueryRequest{Scope: scope, Text: "decision", Mode: "associative"})
	p := matchByID(t, got, target).Reason.Path
	if p.Weight != 2 || p.Source != "baseline" || p.LastActivatedAt == "" {
		t.Fatalf("forgetting lost evidence: %+v", got)
	}
	q.Limit = 1
	first, err := db.FeedbackHistory(q)
	if err != nil || len(first.Events) != 1 || first.Events[0].ID != used.ID || first.NextCursor != used.ID {
		t.Fatalf("first audit page: %+v, %v", first, err)
	}
	q.After = first.NextCursor
	second, err := db.FeedbackHistory(q)
	if err != nil || len(second.Events) != 1 || second.NextCursor != "" || second.Events[0].ID != forgotten.ID ||
		second.Events[0].RecordedAt < first.Events[0].RecordedAt {
		t.Fatalf("next audit page: %+v, %v", second, err)
	}
	// Simultaneous timestamps paginate in ID order, without repeating or losing an event.
	if _, err := db.query("MATCH (f:FeedbackEvent) SET f.recorded_at=$at;", map[string]any{"at": first.Events[0].RecordedAt}); err != nil {
		t.Fatal(err)
	}
	q.After = ""
	first, err = db.FeedbackHistory(q)
	if err != nil || len(first.Events) != 1 || first.NextCursor != min(used.ID, forgotten.ID) {
		t.Fatalf("tied timestamps, first page: %+v, %v", first, err)
	}
	q.After = first.NextCursor
	second, err = db.FeedbackHistory(q)
	if err != nil || len(second.Events) != 1 || second.NextCursor != "" || second.Events[0].ID != max(used.ID, forgotten.ID) {
		t.Fatalf("tied timestamps, next page: %+v, %v", second, err)
	}
	if _, err = db.Delete(protocol.MemoryWriteRequest{Scope: "global", ID: proof, Key: "remove-proof"}); err != nil {
		t.Fatal(err)
	}
	if replay, err := db.Feedback(use); err != nil || !replay.Replayed || replay.ID != used.ID {
		t.Fatalf("replay after deleting cited evidence: %+v, %v", replay, err)
	}
	if _, err = db.Delete(protocol.MemoryWriteRequest{Scope: scope, ID: target, Key: "remove-edge"}); err != nil {
		t.Fatal(err)
	}
	_, err = db.Feedback(use)
	requireCode(t, err, "RESOURCE_DELETED")
	_, err = db.Forget(forget)
	requireCode(t, err, "RESOURCE_DELETED")
}

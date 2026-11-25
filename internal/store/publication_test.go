package store

import (
	"path/filepath"
	"slices"
	"testing"

	lbug "github.com/LadybugDB/go-ladybug"
	"github.com/ewertonls/bigbrain/internal/protocol"
)

func createV7Proposal(t *testing.T, path string) {
	t.Helper()
	createV6Experiences(t, path)
	native, err := lbug.OpenDatabase(path, lbug.DefaultSystemConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer native.Close()
	conn, err := lbug.OpenConnection(native)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for _, statement := range slices.Concat(migrations[6], []string{
		"MATCH (s:BigBrainSchema {name:'foundation'}) SET s.version='7'",
		`MATCH (e:Memory {id:'E1'})
CREATE (p:Memory {id:'P1', scope:'project:A', text:'Private receiver error', source:'A incident',
  recorded_at:'then', memory_type:'lesson_proposal', status:'active', proposal_origin:'external',
  proposal_producer:'operator', proposal_applies_when:'Under load', proposal_rationale:'Quota exceeded'})
CREATE (p)-[:Evidence {id:'R1', scope:'project:A', kind:'derived_from', source:'A incident', weight:1.0,
  created_at:'then', activation_count:0}]->(e)`,
	}) {
		result, err := conn.Query(statement)
		if err != nil {
			t.Fatal(err)
		}
		result.Close()
	}
}

func TestPublishReviewedLesson(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.lbug")
	createV7Proposal(t, path)
	db := openTestStore(t, path)
	const project = "project:A"
	const proposal = "P1"
	migrated := historyForTest(t, db, project, proposal)
	wantMetadata := protocol.Proposal{Origin: "external", Producer: "operator", AppliesWhen: "Under load", Rationale: "Quota exceeded"}
	if migrated.Proposal == nil || *migrated.Proposal != wantMetadata || len(migrated.Related) != 1 ||
		migrated.Related[0].ID != "E1" || migrated.Related[0].Path.RelationshipID != "R1" || migrated.Related[0].Source != "Trial 1" {
		t.Fatalf("v7 proposal lost metadata or attribution: %+v", migrated)
	}
	r := protocol.PublishRequest{Scope: project, Key: "review", ProposalID: proposal, Approve: true,
		Text: "Measure the quota before batching.", Source: "Public review"}

	// Search before publication so the nonempty global index is already cached.
	recordForTest(t, db, protocol.RecordRequest{Scope: "global", Key: "unrelated", Text: "Calendar", Source: "Guide"})
	q := protocol.QueryRequest{Scope: "project:B", IncludeGlobal: true, Text: "quota", Mode: "associative"}
	if found := queryForTest(t, db, q); found.Count != 0 {
		t.Fatalf("lesson visible before approval: %+v", found)
	}
	noApproval := r
	noApproval.Approve = false
	_, err := db.Publish(noApproval)
	requireCode(t, err, "INVALID_REQUEST")

	published, err := db.Publish(r)
	if err != nil || published.Replayed || published.ID == "" || published.ID == proposal {
		t.Fatalf("publish: %+v, %v", published, err)
	}
	found := queryForTest(t, db, q)
	if found.Count != 1 || len(found.Matches) != 1 {
		t.Fatalf("expected one public result: %+v", found)
	}
	lesson := found.Matches[0]
	if lesson.ID != published.ID || lesson.Type != "lesson" || lesson.Scope != "global" ||
		lesson.Text != r.Text || lesson.Source != r.Source || lesson.Reason.Path != nil {
		t.Fatalf("public lesson: %+v", lesson)
	}
	for _, scope := range []string{"global", "project:B", project} {
		h, err := db.History(protocol.HistoryRequest{Scope: scope, IncludeGlobal: scope != "global", ID: lesson.ID})
		if err != nil || h.Memory != lesson.Memory || h.Proposal != nil {
			t.Fatalf("history in %s: %+v, %v", scope, h, err)
		}
		if scope != project {
			if len(h.Related) != 0 {
				t.Fatalf("private publication provenance visible in %s: %+v", scope, h)
			}
			continue
		}
		if len(h.Related) != 1 {
			t.Fatalf("expected publication provenance in A: %+v", h)
		}
		p := h.Related[0].Path
		if p.RelationshipID == "" || p.FromID != lesson.ID || p.ToID != proposal || p.Scope != project ||
			p.Direction != "forward" || p.Kind != "published_from" || p.Source != r.Source {
			t.Fatalf("publication provenance: %+v", p)
		}
	}
	q.Text = "private"
	if got := queryForTest(t, db, q); got.Count != 0 {
		t.Fatalf("private evidence visible from B: %+v", got)
	}
	q.Text, q.IncludeGlobal = "quota", false
	if got := queryForTest(t, db, q); got.Count != 0 {
		t.Fatalf("global lesson visible in B without global inclusion: %+v", got)
	}
	applied := recordForTest(t, db, protocol.RecordRequest{Scope: q.Scope, Key: "application", Text: "Batching succeeded", Source: "Trial B"})
	linkForTest(t, db, protocol.LinkRequest{Scope: q.Scope, Key: "uses", FromID: applied.ID, ToID: lesson.ID,
		Kind: "applies", Weight: 1, Source: "Trial B"})
	_, err = db.Link(protocol.LinkRequest{Scope: q.Scope, Key: "claim-origin", FromID: lesson.ID, ToID: applied.ID,
		Kind: "published_from", Weight: 1, Source: "Trial B"})
	requireCode(t, err, "NOT_FOUND")

	changed := r
	changed.Source = "Different review"
	_, err = db.Publish(changed)
	requireCode(t, err, "IDEMPOTENCY_CONFLICT")
	if _, err := db.Delete(protocol.MemoryWriteRequest{Scope: project, ID: proposal, Key: "remove-draft"}); err != nil {
		t.Fatal(err)
	}
	db.Close()
	db = openTestStore(t, path)
	if got := historyForTest(t, db, "global", lesson.ID); got.Memory != lesson.Memory || len(got.Related) != 0 {
		t.Fatalf("global lesson changed after source deletion and reopen: %+v", got)
	}
	replay, err := db.Publish(r)
	if err != nil || !replay.Replayed || replay.ID != lesson.ID {
		t.Fatalf("retry after source deletion: %+v, %v", replay, err)
	}
	if _, err := db.Delete(protocol.MemoryWriteRequest{Scope: "global", ID: lesson.ID, Key: "remove-lesson"}); err != nil {
		t.Fatal(err)
	}
	_, err = db.Publish(r)
	requireCode(t, err, "RESOURCE_DELETED")
}

func TestPublishRequiresActiveProjectProposal(t *testing.T) {
	db := openTestStore(t, filepath.Join(t.TempDir(), "memory.lbug"))
	const scope = "project:A"
	e := recordForTest(t, db, protocol.RecordRequest{Scope: scope, Key: "experience", Text: "Receiver failed", Source: "Trial"}).ID
	proposal := consolidateForTest(t, db, protocol.ConsolidateRequest{Scope: scope, Key: "draft", EvidenceIDs: []string{e}}).ID
	r := protocol.PublishRequest{Scope: "project:B", Key: "publish", ProposalID: proposal, Approve: true, Text: "Approved", Source: "Review"}
	_, err := db.Publish(r)
	requireCode(t, err, "NOT_FOUND")
	_, err = db.Invalidate(protocol.InvalidateRequest{MemoryWriteRequest: protocol.MemoryWriteRequest{Scope: scope, Key: "retract", ID: proposal},
		Source: "Reviewer", Reason: "Superseded"})
	if err != nil {
		t.Fatal(err)
	}
	r.Scope = scope
	for _, id := range []string{e, proposal, "missing"} {
		r.ProposalID = id
		_, err := db.Publish(r)
		requireCode(t, err, "NOT_FOUND")
	}
	if got := queryForTest(t, db, protocol.QueryRequest{Scope: "global", Text: r.Text}); got.Count != 0 {
		t.Fatalf("rejected publication left a global lesson: %+v", got)
	}
}

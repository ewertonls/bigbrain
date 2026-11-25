package store

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	lbug "github.com/LadybugDB/go-ladybug"
	"github.com/ewertonls/bigbrain/internal/protocol"
)

func createV6Experiences(t *testing.T, path string) {
	t.Helper()
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
	run := func(statement string) {
		t.Helper()
		result, err := conn.Query(statement)
		if err != nil {
			t.Fatal(err)
		}
		result.Close()
	}
	for _, migration := range migrations[:6] {
		for _, statement := range migration {
			run(statement)
		}
	}
	run("MATCH (s:BigBrainSchema {name:'foundation'}) SET s.version='6';")
	run("CREATE (:Memory {id:'E1',scope:'project:A',text:'Receiver throttled.',source:'Trial 1',recorded_at:'then',memory_type:'observation',status:'active'});")
	run("CREATE (:Memory {id:'E2',scope:'project:A',text:'RECEIVER    throttled.',source:'Trial 2',recorded_at:'then',memory_type:'unknown',status:'active'});")
}

func consolidateForTest(t *testing.T, db *Store, r protocol.ConsolidateRequest) protocol.WriteResult {
	t.Helper()
	result, err := db.Consolidate(r)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func historyForTest(t *testing.T, db *Store, scope, id string) protocol.HistoryResult {
	t.Helper()
	result, err := db.History(protocol.HistoryRequest{Scope: scope, ID: id})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestFirstExcerpt(t *testing.T) {
	prefix := strings.Repeat("á", 119)
	for _, tc := range []struct{ input, want string }{
		{"  Use version 2.5. Then retry.", "Use version 2.5."},
		{"The log said “quota reached.” Retry later.", "The log said “quota reached.”"},
		{"First line\r\nSecond line", "First line"},
		{"確認した。次の記録", "確認した。"},
		{"「確認した。」次の記録", "「確認した。」"},
		{prefix + ".」 Next", prefix + "."},
		{prefix + "界more", prefix},
	} {
		if got := firstExcerpt(tc.input); got != tc.want {
			t.Errorf("firstExcerpt(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestConsolidateMigratedExperiences(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.lbug")
	createV6Experiences(t, path)
	db := openTestStore(t, path)
	const scope = "project:A"
	old := historyForTest(t, db, scope, "E1")
	if old.Proposal != nil || old.Memory.Source != "Trial 1" || old.Memory.RecordedAt != "then" {
		t.Fatalf("v6 observation changed: %+v", old)
	}
	q := protocol.QueryRequest{Scope: scope, Text: "receiver", Mode: "text"}
	if got := queryForTest(t, db, q); got.Count != 2 {
		t.Fatalf("v6 observations: %+v", got)
	}
	req := protocol.ConsolidateRequest{Scope: scope, Key: "local-pass", EvidenceIDs: []string{"E2", "E1", "E2"}}
	created := consolidateForTest(t, db, req)
	h := historyForTest(t, db, scope, created.ID)
	if created.Replayed || h.Status != "active" || h.Memory.Type != "lesson_proposal" || h.Memory.Scope != scope ||
		h.Memory.Source != "bigbrain:extractive-v1" || h.Proposal == nil || h.Proposal.Origin != "extractive" ||
		h.Proposal.Producer != h.Memory.Source || h.Proposal.AppliesWhen == "" || h.Proposal.Rationale == "" ||
		!strings.Contains(h.Memory.Text, "Receiver throttled.") || strings.Count(strings.ToLower(h.Memory.Text), "throttled") != 1 ||
		!strings.Contains(h.Memory.Text, "2") {
		t.Fatalf("extractive proposal: %+v, metadata %+v", h.Memory, h.Proposal)
	}
	if len(h.Related) != 2 || h.Related[0].Path.RelationshipID == "" ||
		h.Related[0].Path.RelationshipID >= h.Related[1].Path.RelationshipID {
		t.Fatalf("expected two distinct relationship IDs in history order: %+v", h.Related)
	}
	sources := map[string]string{"E1": "Trial 1", "E2": "Trial 2"}
	for _, related := range h.Related {
		if related.Source != sources[related.ID] || related.Path.FromID != created.ID || related.Path.ToID != related.ID ||
			related.Path.Kind != "derived_from" || related.Path.Direction != "forward" || related.Path.Scope != scope ||
			related.Path.Source != h.Memory.Source || related.Path.Weight != 1 || related.Path.Confidence != nil {
			t.Fatalf("proposal lost evidence provenance: %+v", related)
		}
		delete(sources, related.ID)
	}
	if len(sources) != 0 {
		t.Fatalf("missing evidence: %v", sources)
	}
	if got := queryForTest(t, db, q); got.Count != 3 || matchByID(t, got, created.ID).Source != h.Memory.Source {
		t.Fatalf("cached search did not find both originals and proposal: %+v", got)
	}
	for _, q := range []protocol.QueryRequest{
		{Scope: "global", Text: "receiver"}, {Scope: "project:B", IncludeGlobal: true, Text: "receiver"},
		{Scope: "project:B", IncludeGlobal: true, ID: created.ID},
	} {
		if got := queryForTest(t, db, q); got.Count != 0 {
			t.Fatalf("local proposal visible outside project: %+v", got)
		}
	}
	db.Close()
	db = openTestStore(t, path)
	if got := historyForTest(t, db, scope, created.ID); !reflect.DeepEqual(got, h) {
		t.Fatalf("proposal after reopen: %+v, want %+v", got, h)
	}
	req.EvidenceIDs = []string{"E1", "E2"}
	if replay := consolidateForTest(t, db, req); !replay.Replayed || replay.ID != created.ID {
		t.Fatalf("canonical evidence set did not replay: %+v", replay)
	}
	req.EvidenceIDs = []string{"E1"}
	_, err := db.Consolidate(req)
	requireCode(t, err, "IDEMPOTENCY_CONFLICT")
}

func TestConsolidateValidatesEvidenceAndPreservesExternalProposal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.lbug")
	db := openTestStore(t, path)
	const scope = "project:A"
	record := func(scope, text string) string {
		return recordForTest(t, db, protocol.RecordRequest{Scope: scope, Key: text, Text: text, Source: "trial"}).ID
	}
	active := record(scope, "active experience")
	invalid := record(scope, "invalid experience")
	global := record("global", "shared experience")
	other := record("project:B", "private experience")
	invalidate := func(id string) {
		t.Helper()
		_, err := db.Invalidate(protocol.InvalidateRequest{
			MemoryWriteRequest: protocol.MemoryWriteRequest{Scope: scope, Key: "invalidate-" + id, ID: id},
			Reason:             "retracted", Source: "review",
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	invalidate(invalid)
	req := protocol.ConsolidateRequest{Scope: scope, Key: "draft", EvidenceIDs: []string{active},
		ExternalProposal: &protocol.ExternalProposal{Text: "Reserve quota.", AppliesWhen: "peak load", Rationale: "receiver throttles", Producer: "reviewer"}}
	for _, unavailable := range []string{global, other, invalid, "missing"} {
		bad := req
		bad.EvidenceIDs = []string{active, unavailable}
		_, err := db.Consolidate(bad)
		requireCode(t, err, "NOT_FOUND")
	}
	rows, err := db.query("MATCH (m:Memory {memory_type:'lesson_proposal'}) RETURN m.id;", nil)
	if err != nil || len(rows) != 0 {
		t.Fatalf("invalid evidence left a partial proposal: %v, %v", rows, err)
	}
	created := consolidateForTest(t, db, req)
	h := historyForTest(t, db, scope, created.ID)
	if h.Memory.Text != req.Text || h.Memory.Source != req.Producer || h.Proposal == nil ||
		h.Proposal.Origin != "external" || h.Proposal.Producer != req.Producer ||
		h.Proposal.AppliesWhen != req.AppliesWhen || h.Proposal.Rationale != req.Rationale ||
		len(h.Related) != 1 || h.Related[0].ID != active {
		t.Fatalf("external proposal: %+v, metadata %+v", h, h.Proposal)
	}
	derived := req
	derived.Key, derived.EvidenceIDs = "nested", []string{created.ID}
	_, err = db.Consolidate(derived)
	requireCode(t, err, "NOT_FOUND")
	attributed := req
	draft := *req.ExternalProposal
	attributed.ExternalProposal = &draft
	attributed.Key, attributed.Source = "attributed", "Review 12"
	explicit := historyForTest(t, db, scope, consolidateForTest(t, db, attributed).ID)
	if explicit.Memory.Source != attributed.Source || explicit.Proposal == nil || explicit.Proposal.Producer != req.Producer ||
		len(explicit.Related) != 1 || explicit.Related[0].Path.Source != attributed.Source {
		t.Fatalf("external source and producer: %+v", explicit)
	}
	invalidate(active)
	if got := historyForTest(t, db, scope, created.ID); got.Status != "active" || len(got.Related) != 1 || got.Related[0].Status != "invalidated" {
		t.Fatalf("proposal after invalidation of source: %+v", got)
	}
	retry := func() {
		t.Helper()
		if replay := consolidateForTest(t, db, req); !replay.Replayed || replay.ID != created.ID {
			t.Fatalf("retry after evidence changed: %+v", replay)
		}
	}
	retry()
	remove := func(id string) {
		t.Helper()
		if _, err := db.Delete(protocol.MemoryWriteRequest{Scope: scope, ID: id, Key: "delete-" + id}); err != nil {
			t.Fatal(err)
		}
	}
	remove(active)
	retry()
	if got := historyForTest(t, db, scope, created.ID); got.Status != "active" || len(got.Related) != 0 {
		t.Fatalf("retry restored deleted evidence: %+v", got)
	}
	remove(created.ID)
	db.Close()
	db = openTestStore(t, path)
	_, err = db.Consolidate(req)
	requireCode(t, err, "RESOURCE_DELETED")
}

func TestLinkPreservesProposalEvidence(t *testing.T) {
	db := openTestStore(t, filepath.Join(t.TempDir(), "memory.lbug"))
	const scope = "project:A"
	original := recordForTest(t, db, protocol.RecordRequest{Scope: scope, Key: "original", Text: "First trial", Source: "Report 1"}).ID
	later := recordForTest(t, db, protocol.RecordRequest{Scope: scope, Key: "later", Text: "Later trial", Source: "Report 2"}).ID
	proposal := consolidateForTest(t, db, protocol.ConsolidateRequest{Scope: scope, Key: "lesson", EvidenceIDs: []string{original}}).ID
	link := protocol.LinkRequest{Scope: scope, Key: "follow-up", FromID: proposal, ToID: later, Kind: "derived_from", Source: "Review", Weight: 1}
	_, err := db.Link(link)
	requireCode(t, err, "NOT_FOUND")

	link.Kind = "related_to"
	linkForTest(t, db, link)
	link.Key, link.Kind, link.FromID, link.ToID = "reverse", "derived_from", later, proposal
	linkForTest(t, db, link)
	history := historyForTest(t, db, scope, proposal)
	var evidence []string
	for _, related := range history.Related {
		if related.Path.Kind == "derived_from" && related.Path.Direction == "forward" {
			evidence = append(evidence, related.ID)
		}
	}
	if len(history.Related) != 3 || len(evidence) != 1 || evidence[0] != original {
		t.Fatalf("proposal's original evidence changed: %+v", history.Related)
	}
}

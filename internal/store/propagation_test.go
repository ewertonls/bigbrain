package store

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

type propagationFixture struct {
	t  *testing.T
	db *Store
}

func (g propagationFixture) record(scope, text string) string {
	g.t.Helper()
	return recordForTest(g.t, g.db, protocol.RecordRequest{Scope: scope, Key: text, Text: text, Source: text + " source"}).ID
}

func (g propagationFixture) link(scope, key, from, to string, weight float64, confidence *float64) string {
	g.t.Helper()
	return linkForTest(g.t, g.db, protocol.LinkRequest{Scope: scope, Key: key, FromID: from, ToID: to,
		Kind: "cites", Source: key, Weight: weight, Confidence: confidence}).ID
}

func matchByID(t *testing.T, result protocol.QueryResult, id string) protocol.Match {
	t.Helper()
	for _, m := range result.Matches {
		if m.ID == id {
			return m
		}
	}
	t.Fatalf("missing %s in %+v", id, result)
	return protocol.Match{}
}

func requireSteps(t *testing.T, reason protocol.Reason, ids ...string) {
	t.Helper()
	if reason.Hops != len(ids) || len(reason.Steps) != len(ids) {
		t.Fatalf("got %d hops and steps %+v, want %v", reason.Hops, reason.Steps, ids)
	}
	for i, id := range ids {
		if reason.Steps[i].RelationshipID != id {
			t.Fatalf("step %d: got %+v, want relationship %s", i, reason.Steps[i], id)
		}
	}
	if !reflect.DeepEqual(reason.Path, &reason.Steps[len(ids)-1]) {
		t.Fatalf("terminal path %+v differs from last step %+v", reason.Path, reason.Steps[len(ids)-1])
	}
}

func TestPropagationBoundsAndTies(t *testing.T) {
	g := propagationFixture{t, openTestStore(t, filepath.Join(t.TempDir(), "memory.lbug"))}
	const scope = "project:A"
	seed := g.record(scope, "orbit")
	left, right := g.record(scope, "left"), g.record(scope, "right")
	end := g.record(scope, "evidence")
	g.link(scope, "self", seed, seed, 100, nil)
	toLeft := g.link(scope, "first left", seed, left, 5, nil)
	alsoLeft := g.link(scope, "second left", seed, left, 5, nil)
	toRight := g.link(scope, "to right", seed, right, 5, nil)
	viaLeft := g.link(scope, "left proof", end, left, 10, nil)
	viaRight := g.link(scope, "right proof", end, right, 10, nil)

	q := protocol.QueryRequest{Scope: scope, Text: "orbit", Mode: "associative", MaxNeighbors: new(1)}
	got := queryForTest(t, g.db, q)
	if got.Count != 2 || got.Matches[1].ID != min(left, right) || len(got.Matches[1].Reason.Steps) != 0 {
		t.Fatalf("default depth, one neighbor: %+v", got)
	}
	q.MaxNeighbors = new(2)
	got = queryForTest(t, g.db, q)
	if got.Count != 3 || got.Matches[1].ID != min(left, right) || got.Matches[2].ID != max(left, right) {
		t.Fatalf("parallel links consumed neighbor slots: %+v", got)
	}
	// Both two-hop routes have equal strength. The first relationship ID
	// distinguishes them, independently of the order of their final edges.
	first, last := toRight, viaRight
	if min(toLeft, alsoLeft) < toRight {
		first, last = min(toLeft, alsoLeft), viaLeft
	}
	q.MaxDepth = new(2)
	got = queryForTest(t, g.db, q)
	reason := matchByID(t, got, end).Reason
	requireSteps(t, reason, first, last)
	if got.Count != 4 || reason.Path.Direction != "reverse" {
		t.Fatalf("chosen path in diamond: %+v", got)
	}
	q.MaxDepth = new(3)
	if more := queryForTest(t, g.db, q); !reflect.DeepEqual(more, got) {
		t.Fatalf("extra depth introduced a revisit: %+v", more)
	}
}

func TestPropagationUsesStrongerRouteWithinDepthBudget(t *testing.T) {
	g := propagationFixture{t, openTestStore(t, filepath.Join(t.TempDir(), "memory.lbug"))}
	const scope = "project:A"
	seed := g.record(scope, "orbit")
	bridge, junction, end := g.record(scope, "bridge"), g.record(scope, "junction"), g.record(scope, "evidence")
	shortcut := g.link(scope, "shortcut", seed, junction, 0.01, nil)
	first := g.link(scope, "first", seed, bridge, 10, nil)
	second := g.link(scope, "second", bridge, junction, 10, nil)
	third := g.link(scope, "third", junction, end, 10, nil)

	q := protocol.QueryRequest{Scope: scope, Text: "orbit", Mode: "associative", MaxDepth: new(2)}
	got := queryForTest(t, g.db, q)
	near := matchByID(t, got, end).Reason
	if got.Count != 4 {
		t.Fatalf("expected four memories including the evidence: %+v", got)
	}
	// The best path to the junction cannot serve as the prefix of the best
	// path to the evidence within this depth limit.
	requireSteps(t, matchByID(t, got, junction).Reason, first, second)
	requireSteps(t, near, shortcut, third)
	q.MaxDepth = new(3)
	far := matchByID(t, queryForTest(t, g.db, q), end).Reason
	requireSteps(t, far, first, second, third)
	if far.GraphScore <= near.GraphScore {
		t.Fatalf("larger depth failed to propagate stronger activation: %+v -> %+v", near, far)
	}
}

func TestPropagationTiedRoutesWithReversedWeights(t *testing.T) {
	g := propagationFixture{t, openTestStore(t, filepath.Join(t.TempDir(), "memory.lbug"))}
	const scope = "project:A"
	seed, end := g.record(scope, "orbit"), g.record(scope, "evidence")
	left, right := g.record(scope, "left"), g.record(scope, "right")
	for _, edge := range []struct {
		id, from, to string
		weight       float64
	}{
		{"A", seed, left, 3}, {"Z", left, end, 1},
		{"B", seed, right, 1}, {"C", right, end, 3},
	} {
		_, err := g.db.query(`MATCH (a:Memory {id:$from}), (b:Memory {id:$to})
CREATE (a)-[:Evidence {id:$id, scope:$scope, kind:'cites', source:'review', weight:$weight}]->(b)`,
			map[string]any{"from": edge.from, "to": edge.to, "id": edge.id, "scope": scope, "weight": edge.weight})
		if err != nil {
			t.Fatal(err)
		}
	}
	// A-Z and B-C apply the same factors. Lexical order selects A-Z, even
	// though its terminal ID sorts later than C.
	result := queryForTest(t, g.db, protocol.QueryRequest{Scope: scope, Text: "orbit", MaxDepth: new(2)})
	if reason := matchByID(t, result, end).Reason; reason.Hops != 2 || reason.Path == nil || reason.Path.RelationshipID != "Z" {
		t.Fatalf("expected A-Z for mathematically equal routes, got %d hops and final edge %+v", reason.Hops, reason.Path)
	}
}

func TestPropagationRanksBySeedContextAndConfidence(t *testing.T) {
	g := propagationFixture{t, openTestStore(t, filepath.Join(t.TempDir(), "memory.lbug"))}
	const scope = "project:A"
	satellite, antenna := g.record(scope, "radio satellite"), g.record(scope, "radio antenna")
	first, second, uncertain := g.record(scope, "first"), g.record(scope, "second"), g.record(scope, "uncertain")
	g.link(scope, "satellite review", satellite, first, 5, new(1.0))
	g.link(scope, "antenna review", antenna, second, 5, new(1.0))
	g.link(scope, "weak evidence", satellite, uncertain, 5, new(0.1))
	for _, tc := range []struct{ text, stronger, weaker string }{
		{"radio satellite", first, second}, {"radio antenna", second, first},
	} {
		got := queryForTest(t, g.db, protocol.QueryRequest{Scope: scope, Text: tc.text, Mode: "associative", MaxDepth: new(2)})
		if got.Count != 5 || got.Matches[2].ID != tc.stronger ||
			matchByID(t, got, tc.stronger).Reason.GraphScore <= matchByID(t, got, tc.weaker).Reason.GraphScore ||
			matchByID(t, got, first).Reason.GraphScore <= matchByID(t, got, uncertain).Reason.GraphScore {
			t.Fatalf("context and confidence for %q: %+v", tc.text, got)
		}
	}
}

func TestPropagationScopeInvalidationAndHybridFallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.lbug")
	model := filepath.Join(dir, "model.txt")
	if err := os.WriteFile(model, []byte("2 2\ngateway 1 0\nportal 1 0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BIGBRAIN_EMBEDDINGS", model)
	t.Setenv("BIGBRAIN_VECTOR_INDEX", "")
	g := propagationFixture{t, openTestStore(t, path)}
	seed := g.record("global", "gateway")
	middleB := g.record("project:B", "private finding B")
	end := g.record("global", "published evidence")
	entryB := g.link("project:B", "B entry", seed, middleB, 5, nil)
	evidenceB := g.link("project:B", "B evidence", end, middleB, 10, new(0.0))
	detour := g.record("project:B", "unconfirmed lead")
	g.link("project:B", "B detour", middleB, detour, 0.1, nil)
	handoff := g.record("project:B", "handoff")
	g.link("project:B", "B handoff", handoff, seed, 0.01, nil)
	q := protocol.QueryRequest{Scope: "project:B", IncludeGlobal: true, Text: "portal", Mode: "hybrid", MaxDepth: new(2), MaxNeighbors: new(1)}
	before := queryForTest(t, g.db, q)
	evidence := matchByID(t, before, end)
	requireSteps(t, evidence.Reason, entryB, evidenceB)
	first, last := evidence.Reason.Steps[0], evidence.Reason.Steps[1]
	oneHop := matchByID(t, before, middleB).Reason
	if len(oneHop.Steps) != 0 || !reflect.DeepEqual(oneHop.Path, &first) {
		t.Fatalf("one-hop path and first step differ: %+v, %+v", oneHop, first)
	}
	if evidence.Source != "published evidence source" || evidence.Scope != "global" ||
		first.FromID != seed || first.ToID != middleB || first.Direction != "forward" || first.Source != "B entry" ||
		last.FromID != end || last.ToID != middleB || last.Direction != "reverse" || last.Source != "B evidence" ||
		first.Scope != q.Scope || last.Scope != q.Scope || first.Kind != "cites" || last.Kind != "cites" {
		t.Fatalf("evidence provenance or mixed-direction path: %+v via %+v", evidence.Memory, evidence.Reason.Steps)
	}
	if before.Fallback != "" || before.Count != 3 ||
		matchByID(t, before, seed).Reason.GraphScore != 0 {
		t.Fatalf("semantic seed and two-hop evidence: %+v", before)
	}
	wider := q
	wider.MaxNeighbors = new(3)
	if got := queryForTest(t, g.db, wider); got.Count != 5 || matchByID(t, got, detour).Reason.Hops != 2 {
		t.Fatalf("neighbor limit at intermediate node: %+v", got)
	}
	// Starting at handoff makes the shared gateway the intermediate node.
	local := q
	local.Text, local.Mode = "handoff", "associative"
	localBefore := queryForTest(t, g.db, local)
	if localBefore.Count != 3 || matchByID(t, localBefore, middleB).Reason.Hops != 2 {
		t.Fatalf("B route through global gateway: %+v", localBefore)
	}
	middleA := g.record("project:A", "private finding A")
	g.link("project:A", "A entry", seed, middleA, 100, nil)
	g.link("project:A", "A evidence", end, middleA, 200, nil)
	if after := queryForTest(t, g.db, q); !reflect.DeepEqual(after, before) {
		t.Fatalf("private links changed B: %+v -> %+v", before, after)
	}
	if after := queryForTest(t, g.db, local); !reflect.DeepEqual(after, localBefore) {
		t.Fatalf("private link at intermediate gateway changed B: %+v -> %+v", localBefore, after)
	}
	q.Scope = "project:A"
	got := queryForTest(t, g.db, q)
	if got.Count != 3 || matchByID(t, got, end).Reason.Hops != 2 {
		t.Fatalf("A's route before invalidation: %+v", got)
	}
	_, err := g.db.Invalidate(protocol.InvalidateRequest{
		MemoryWriteRequest: protocol.MemoryWriteRequest{Scope: q.Scope, Key: "withdraw", ID: middleA},
		Reason:             "superseded", Source: "review",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := queryForTest(t, g.db, q); got.Count != 1 || got.Matches[0].ID != seed {
		t.Fatalf("invalidated or invisible intermediate traversed: %+v", got)
	}

	q.Scope, q.Mode, q.Text = "project:B", "associative", "gateway"
	lexical := queryForTest(t, g.db, q)
	if lexical.Count != 3 || !reflect.DeepEqual(matchByID(t, lexical, end).Reason.Steps, evidence.Reason.Steps) {
		t.Fatalf("B's lexical route: %+v", lexical)
	}
	g.db.Close()
	t.Setenv("BIGBRAIN_VECTOR_INDEX", "off")
	g.db = openTestStore(t, path)
	q.Mode = "hybrid"
	fallback := queryForTest(t, g.db, q)
	if fallback.Fallback != "vector_index_disabled" || fallback.Count != lexical.Count || !reflect.DeepEqual(fallback.Matches, lexical.Matches) {
		t.Fatalf("lexical fallback: %+v, want %+v", fallback, lexical)
	}
}

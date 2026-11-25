package store

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	lbug "github.com/LadybugDB/go-ladybug"
	"github.com/ewertonls/bigbrain/internal/protocol"
)

func TestWriteReceiptsSurviveReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bigbrain.lbug")
	database := openTestStore(t, path)
	request := protocol.RecordRequest{Scope: "project:A", Key: "decision", Text: "Orbit sends readings in batches.", Source: "Decision 12"}
	decision := recordForTest(t, database, request)
	evidence := recordForTest(t, database, protocol.RecordRequest{Scope: "project:A", Key: "evidence", Text: "The receiver accepts only one request per minute.", Source: "Test 7"})
	link := protocol.LinkRequest{
		Scope: "project:A", Key: "link", FromID: decision.ID, ToID: evidence.ID, Kind: "justified_by", Source: "Review of Decision 12", Weight: 1,
	}
	relationship := linkForTest(t, database, link)
	if decision.Replayed || relationship.Replayed {
		t.Fatal("new writes were reported as replays")
	}
	database.Close()
	database = openTestStore(t, path)

	if retry := recordForTest(t, database, request); retry.ID != decision.ID || !retry.Replayed {
		t.Fatalf("record replay = %+v, original = %+v", retry, decision)
	}
	if retry := linkForTest(t, database, link); retry.ID != relationship.ID || !retry.Replayed {
		t.Fatalf("link replay = %+v, original = %+v", retry, relationship)
	}
	changed := request
	changed.Text = "Orbit sends each reading."
	_, err := database.Record(changed)
	requireCode(t, err, "IDEMPOTENCY_CONFLICT")
	changed = request
	changed.Scope = "project:B"
	_, err = database.Record(changed)
	requireCode(t, err, "IDEMPOTENCY_CONFLICT")
	link.Key = request.Key
	_, err = database.Link(link)
	requireCode(t, err, "IDEMPOTENCY_CONFLICT")

	matches := queryForTest(t, database, protocol.QueryRequest{Scope: "project:A", Text: "receiver"})
	if matches.Count != 2 || matches.Matches[0].ID != evidence.ID || matches.Matches[1].ID != decision.ID {
		t.Fatalf("unexpected reverse matches: %+v", matches)
	}
	pathResult := matches.Matches[1].Reason.Path
	if pathResult == nil || pathResult.RelationshipID != relationship.ID || pathResult.FromID != decision.ID ||
		pathResult.ToID != evidence.ID || pathResult.Direction != "reverse" || pathResult.Source != link.Source {
		t.Fatalf("unexpected reverse path: %+v", pathResult)
	}
}

func TestScopedNeighborsRankAndDeduplicate(t *testing.T) {
	database := openTestStore(t, filepath.Join(t.TempDir(), "bigbrain.lbug"))
	record := func(scope, key, text string) string {
		return recordForTest(t, database, protocol.RecordRequest{Scope: scope, Key: key, Text: text, Source: "Source " + key}).ID
	}
	anchor := record("project:A", "anchor", "orbit sends readings")
	ranked := []string{anchor}
	for weight := 7; weight > 0; weight-- {
		key := fmt.Sprint(weight)
		neighbor := record("project:A", key, "neighbor "+key)
		link := protocol.LinkRequest{
			Scope: "project:A", Key: "link" + key, FromID: anchor, ToID: neighbor,
			Kind: "related", Source: "Relationship " + key, Weight: float64(weight),
		}
		if weight%2 == 0 {
			link.FromID, link.ToID = link.ToID, link.FromID
		}
		linkForTest(t, database, link)
		if weight == 7 {
			link.Key += "duplicate"
			link.Weight = 6.5
			linkForTest(t, database, link)
		}
		if len(ranked) <= neighborsPerMatch {
			ranked = append(ranked, neighbor)
		}
	}
	other := record("project:B", "other", "orbit sends readings")
	global := record("global", "global", "orbit sends readings")
	badLink := protocol.LinkRequest{Scope: "project:A", Key: "bad", FromID: anchor, ToID: other, Kind: "related", Source: "Cross scope"}
	_, err := database.Link(badLink)
	requireCode(t, err, "NOT_FOUND")
	badLink.ToID = global
	linkForTest(t, database, badLink)

	query := protocol.QueryRequest{Scope: "project:A", Text: "Orbit"}
	matches := queryForTest(t, database, query)
	ids := make([]string, 0, len(matches.Matches))
	for _, match := range matches.Matches {
		ids = append(ids, match.ID)
	}
	if !slices.Equal(ids, ranked) || matches.Count != neighborsPerMatch+1 {
		t.Fatalf("ranked IDs = %v, want %v", ids, ranked)
	}
	query.IncludeGlobal = true
	query.Mode = "text"
	query.Limit = 1
	matches = queryForTest(t, database, query)
	if matches.Count != 2 || len(matches.Matches) != 1 {
		t.Fatalf("global text lookup: %+v", matches)
	}
	query.Text = ""
	query.ID = other
	matches = queryForTest(t, database, query)
	if matches.Count != 0 || len(matches.Matches) != 0 {
		t.Fatalf("other project visible by ID: %+v", matches)
	}
	query.ID = global
	matches = queryForTest(t, database, query)
	if matches.Count != 1 || len(matches.Matches) != 1 || matches.Matches[0].ID != global {
		t.Fatalf("included global record unavailable by ID: %+v", matches)
	}
}

func TestOpenPreservesUnrecognizedDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unrecognized.lbug")
	createUnrecognizedDatabase(t, path)

	_, err := Open(path)
	requireCode(t, err, "DATABASE_INCOMPATIBLE")
	database, err := lbug.OpenDatabase(path, lbug.DefaultSystemConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	connection, err := lbug.OpenConnection(database)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	result, err := connection.Query("CALL SHOW_TABLES() RETURN name;")
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	tuple, err := result.Next()
	if err != nil {
		t.Fatal(err)
	}
	defer tuple.Close()
	table, err := tuple.GetValue(0)
	if err != nil || table != "Unrelated" || result.HasNext() {
		t.Fatalf("catalog changed: first table %q, error %v", table, err)
	}
}

func createUnrecognizedDatabase(t *testing.T, path string) {
	t.Helper()
	database, err := lbug.OpenDatabase(path, lbug.DefaultSystemConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	connection, err := lbug.OpenConnection(database)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	result, err := connection.Query("CREATE NODE TABLE Unrelated(id STRING, PRIMARY KEY(id));")
	if err != nil {
		t.Fatal(err)
	}
	result.Close()
}

func openTestStore(t *testing.T, path string) *Store {
	t.Helper()
	database, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(database.Close)
	return database
}

func recordForTest(t *testing.T, database *Store, request protocol.RecordRequest) protocol.WriteResult {
	t.Helper()
	result, err := database.Record(request)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func linkForTest(t *testing.T, database *Store, request protocol.LinkRequest) protocol.WriteResult {
	t.Helper()
	result, err := database.Link(request)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func queryForTest(t *testing.T, database *Store, request protocol.QueryRequest) protocol.QueryResult {
	t.Helper()
	result, err := database.Query(request)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	var operationError *protocol.Error
	if !errors.As(err, &operationError) || operationError.Code != code {
		t.Fatalf("error = %v, want code %s", err, code)
	}
}

func TestProjectGlobalPathsRespectOwnerAndEndpoints(t *testing.T) {
	db := openTestStore(t, filepath.Join(t.TempDir(), "memory.lbug"))
	record := func(scope, text string) string {
		return recordForTest(t, db, protocol.RecordRequest{Scope: scope, Key: text, Text: text, Source: text + " source"}).ID
	}
	a, b, g := record("project:A", "alpha"), record("project:B", "beta"), record("global", "shared")
	link := func(scope, key, from, to string, weight float64) protocol.LinkRequest {
		return protocol.LinkRequest{Scope: scope, Key: key, FromID: from, ToID: to, Kind: "cites", Source: key, Weight: weight}
	}
	l := linkForTest(t, db, link("project:A", "A review", g, a, 1))
	linkForTest(t, db, link("project:B", "B review", b, g, 100))
	for _, test := range []struct {
		scope, text, neighbor, direction string
		global                           bool
	}{
		{"global", "shared", "", "", false},
		{"project:A", "alpha", "", "", false},
		{"project:A", "shared", a, "forward", true},
		{"project:A", "alpha", g, "reverse", true},
		{"project:B", "shared", b, "reverse", true},
	} {
		r := queryForTest(t, db, protocol.QueryRequest{Scope: test.scope, Text: test.text, Mode: "associative", IncludeGlobal: test.global})
		if test.neighbor == "" {
			if r.Count != 1 {
				t.Fatalf("private path exposed to %+v: %+v", test, r)
			}
			continue
		}
		if r.Count != 2 || r.Matches[1].ID != test.neighbor {
			t.Fatalf("%+v: %+v", test, r)
		}
		p := r.Matches[1].Reason.Path
		if p == nil || p.Scope != test.scope || p.Direction != test.direction {
			t.Fatalf("bad path: %+v", p)
		}
		if test.scope == "project:A" && (p.RelationshipID != l.ID || p.FromID != g || p.ToID != a || p.Source != "A review" || p.Weight != 1) {
			t.Fatalf("private link influenced ranking: %+v", p)
		}
	}
	for _, request := range []protocol.LinkRequest{
		link("project:A", "AB", a, b, 1), link("project:B", "BA", b, a, 1),
		link("global", "GA", g, a, 1), link("project:A", "GG", g, g, 1),
	} {
		_, err := db.Link(request)
		requireCode(t, err, "NOT_FOUND")
	}
}

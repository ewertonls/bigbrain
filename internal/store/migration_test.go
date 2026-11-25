package store

import (
	"path/filepath"
	"testing"

	lbug "github.com/LadybugDB/go-ladybug"
	"github.com/ewertonls/bigbrain/internal/protocol"
)

func TestFoundationMigrationAndTypedRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.lbug")
	native, err := lbug.OpenDatabase(path, lbug.DefaultSystemConfig())
	if err != nil {
		t.Fatal(err)
	}
	conn, err := lbug.OpenConnection(native)
	if err != nil {
		t.Fatal(err)
	}
	// The stored receipts fingerprint the original Foundation request JSON.
	for _, statement := range []string{
		"CREATE NODE TABLE BigBrainSchema(name STRING, version STRING, PRIMARY KEY(name));",
		"CREATE NODE TABLE Memory(id STRING, scope STRING, text STRING, source STRING, recorded_at STRING, PRIMARY KEY(id));",
		"CREATE NODE TABLE Receipt(key STRING, operation STRING, scope STRING, fingerprint STRING, result_id STRING, PRIMARY KEY(key));",
		"CREATE REL TABLE Evidence(FROM Memory TO Memory, id STRING, kind STRING, weight DOUBLE, source STRING);",
		"CREATE (:BigBrainSchema {name:'foundation', version:'1'});",
		"CREATE (:Memory {id:'D1', scope:'project:A', text:'Orbit', source:'Decision 12', recorded_at:'then'});",
		"CREATE (:Memory {id:'E1', scope:'project:A', text:'receiver', source:'Test 7', recorded_at:'then'});",
		"MATCH (d:Memory {id:'D1'}), (e:Memory {id:'E1'}) CREATE (d)-[:Evidence {id:'R1',kind:'justified_by',source:'Review',weight:1.0}]->(e);",
		"CREATE (:Receipt {key:'saved',operation:'record',scope:'project:A',fingerprint:'2d0eb1f86e6e5ba3486715a2ef100c35086a7454e4463d8e709e1689b9e16a5e',result_id:'D1'});",
		"CREATE (:Receipt {key:'saved-link',operation:'link',scope:'project:A',fingerprint:'af9894b73e05d7b65715646a7cd8de68c88bc71d0c7e328587f79871caef0df8',result_id:'R1'});",
	} {
		result, err := conn.Query(statement)
		if err != nil {
			t.Fatal(err)
		}
		result.Close()
	}
	conn.Close()
	native.Close()

	db := openTestStore(t, path)
	old := protocol.RecordRequest{Scope: "project:A", Key: "saved", Text: "Orbit", Source: "Decision 12"}
	if replay := recordForTest(t, db, old); !replay.Replayed || replay.ID != "D1" {
		t.Fatalf("old receipt: %+v", replay)
	}
	old.Type = "decision"
	_, err = db.Record(old)
	requireCode(t, err, "IDEMPOTENCY_CONFLICT")
	link := protocol.LinkRequest{Scope: "project:A", Key: "saved-link", FromID: "D1", ToID: "E1", Kind: "justified_by", Source: "Review", Weight: 1}
	if replay := linkForTest(t, db, link); !replay.Replayed || replay.ID != "R1" {
		t.Fatalf("old link receipt: %+v", replay)
	}
	for _, tc := range []struct{ text, seed, neighbor, source, direction string }{
		{"Orbit", "D1", "E1", "Test 7", "forward"},
		{"receiver", "E1", "D1", "Decision 12", "reverse"},
	} {
		got := queryForTest(t, db, protocol.QueryRequest{Scope: "project:A", Text: tc.text, Mode: "associative"}).Matches
		if len(got) != 2 || got[0].ID != tc.seed || got[0].Type != "unknown" || got[0].RecordedAt != "then" ||
			got[1].ID != tc.neighbor || got[1].Source != tc.source {
			t.Fatalf("migrated %s matches: %+v", tc.direction, got)
		}
		// Foundation did not record confidence, activations or creation time.
		want := protocol.Path{Scope: link.Scope, RelationshipID: "R1", FromID: link.FromID, ToID: link.ToID,
			Kind: link.Kind, Source: link.Source, Weight: link.Weight, Direction: tc.direction}
		if path := got[1].Reason.Path; path == nil || *path != want {
			t.Fatalf("migrated %s path: got %+v, want %+v", tc.direction, path, want)
		}
	}
	ids := map[string]string{}
	for _, kind := range []string{"semantic", "episodic", "procedural", "decision", "preference", "entity", "observation"} {
		record := protocol.RecordRequest{Scope: "project:A", Key: kind, Type: kind, Text: "Typed memory", Source: "Session"}
		ids[kind] = recordForTest(t, db, record).ID
	}
	db.Close()
	db = openTestStore(t, path)
	for kind, id := range ids {
		got := queryForTest(t, db, protocol.QueryRequest{Scope: "project:A", ID: id}).Matches
		if len(got) != 1 || got[0].Type != kind || got[0].Source != "Session" {
			t.Fatalf("%s: %+v", kind, got)
		}
	}
	old.Key, old.Type = "invalid", "mystery"
	_, err = db.Record(old)
	requireCode(t, err, "INVALID_REQUEST")
}

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
	// This fixture uses the Foundation on-disk and JSON contracts. It must
	// remain readable even when the current schema and record request evolve.
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
	want := protocol.QueryRequest{Scope: "project:A", Text: "Orbit", Mode: "associative"}
	got := queryForTest(t, db, want).Matches
	if len(got) != 2 || got[0].ID != "D1" || got[0].Type != "unknown" || got[0].RecordedAt != "then" ||
		got[1].ID != "E1" || got[1].Source != "Test 7" ||
		got[1].Reason.Path == nil || got[1].Reason.Path.RelationshipID != "R1" || got[1].Reason.Path.Source != "Review" {
		t.Fatalf("migrated graph: %+v", got)
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

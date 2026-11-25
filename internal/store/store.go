package store

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	lbug "github.com/LadybugDB/go-ladybug"
	"github.com/ewertonls/bigbrain/internal/protocol"
)

const schemaVersion = "1"

// Store owns one native connection. The daemon serializes access to it.
type Store struct {
	database   *lbug.Database
	connection *lbug.Connection

	// The process-level acceptance tests use these transaction boundaries.
	BeforeCommit func()
	AfterCommit  func()
}

func Open(path string) (*Store, error) {
	info, err := os.Lstat(path)
	switch {
	case err == nil && (!info.Mode().IsRegular() || info.Size() == 0):
		return nil, protocol.NewError("DATABASE_INCOMPATIBLE", "The database path does not contain a LadybugDB file.")
	case err != nil && !errors.Is(err, os.ErrNotExist):
		return nil, protocol.NewError("DATABASE_UNAVAILABLE", "Cannot access the database file: "+err.Error())
	}

	config := lbug.DefaultSystemConfig()
	config.MaxNumThreads = 2
	database, err := lbug.OpenDatabase(path, config)
	if err != nil {
		// The binding returns a handle even when initialization fails.
		if database != nil {
			database.Close()
		}
		return nil, protocol.NewError("DATABASE_UNAVAILABLE", "LadybugDB could not open the database. Check its permissions, format, and whether another process has it open.")
	}
	connection, err := lbug.OpenConnection(database)
	if err != nil {
		if connection != nil {
			connection.Close()
		}
		database.Close()
		return nil, protocol.NewError("DATABASE_UNAVAILABLE", "Could not connect to LadybugDB: "+err.Error())
	}
	store := &Store{database: database, connection: connection}
	if err := store.openSchema(); err != nil {
		store.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) openSchema() error {
	tables, err := s.query("CALL SHOW_TABLES() RETURN name;", nil)
	if err != nil {
		return protocol.NewError("DATABASE_UNAVAILABLE", "Could not read the LadybugDB catalog: "+err.Error())
	}
	if len(tables) == 0 {
		if err := s.createSchema(); err != nil {
			return protocol.NewError("DATABASE_UNAVAILABLE", "Could not initialize the BigBrain schema: "+err.Error())
		}
		return nil
	}
	version, err := s.query("MATCH (schema:BigBrainSchema {name: 'foundation'}) RETURN schema.version;", nil)
	if err != nil || len(version) != 1 || version[0][0] != schemaVersion {
		return protocol.NewError("DATABASE_INCOMPATIBLE", "The database does not use the BigBrain Foundation schema.")
	}

	// Binding these column names also detects a partial or damaged catalog.
	for _, statement := range []string{
		"MATCH (m:Memory) RETURN m.id, m.scope, m.text, m.source, m.recorded_at LIMIT 0;",
		"MATCH (r:Receipt) RETURN r.key, r.operation, r.scope, r.fingerprint, r.result_id LIMIT 0;",
		"MATCH (:Memory)-[r:Evidence]->(:Memory) RETURN r.id, r.kind, r.weight, r.source LIMIT 0;",
	} {
		if _, err := s.query(statement, nil); err != nil {
			return protocol.NewError("DATABASE_INCOMPATIBLE", "The database is missing part of the BigBrain Foundation schema: "+err.Error())
		}
	}
	return nil
}

func (s *Store) createSchema() error {
	if _, err := s.query("BEGIN TRANSACTION;", nil); err != nil {
		return err
	}
	committed := false
	defer s.rollbackUnlessCommitted(&committed)

	for _, statement := range []string{
		"CREATE NODE TABLE BigBrainSchema(name STRING, version STRING, PRIMARY KEY(name));",
		"CREATE NODE TABLE Memory(id STRING, scope STRING, text STRING, source STRING, recorded_at STRING, PRIMARY KEY(id));",
		"CREATE NODE TABLE Receipt(key STRING, operation STRING, scope STRING, fingerprint STRING, result_id STRING, PRIMARY KEY(key));",
		"CREATE REL TABLE Evidence(FROM Memory TO Memory, id STRING, kind STRING, weight DOUBLE, source STRING);",
	} {
		if _, err := s.query(statement, nil); err != nil {
			return err
		}
	}
	if _, err := s.query("CREATE (:BigBrainSchema {name: 'foundation', version: $version});", map[string]any{"version": schemaVersion}); err != nil {
		return err
	}
	if _, err := s.query("COMMIT;", nil); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *Store) Close() {
	s.connection.Close()
	s.database.Close()
}

func (s *Store) Record(request protocol.RecordRequest) (protocol.WriteResult, error) {
	if err := request.Validate(); err != nil {
		return protocol.WriteResult{}, err
	}
	return s.write("record", request.Scope, request.Key, request, func(id string) error {
		_, err := s.query(`CREATE (:Memory {id: $id, scope: $scope, text: $text, source: $source, recorded_at: $recordedAt});`, map[string]any{
			"id": id, "scope": request.Scope, "text": request.Text, "source": request.Source,
			"recordedAt": time.Now().UTC().Format(time.RFC3339Nano),
		})
		return err
	})
}

func (s *Store) Link(request protocol.LinkRequest) (protocol.WriteResult, error) {
	if err := request.Validate(); err != nil {
		return protocol.WriteResult{}, err
	}
	// Negative zero represents the same weight as positive zero.
	if request.Weight == 0 {
		request.Weight = 0
	}
	return s.write("link", request.Scope, request.Key, request, func(id string) error {
		rows, err := s.query(`
MATCH (a:Memory {id: $from}), (b:Memory {id: $to})
WHERE a.scope = $scope AND b.scope = $scope
CREATE (a)-[:Evidence {id: $id, kind: $kind, weight: $weight, source: $source}]->(b)
RETURN a.id;`, map[string]any{
			"from": request.FromID, "to": request.ToID, "scope": request.Scope, "id": id,
			"kind": request.Kind, "weight": request.Weight, "source": request.Source,
		})
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return protocol.NewError("NOT_FOUND", "Both memories must exist in the selected scope.")
		}
		return nil
	})
}

func (s *Store) write(operation, scope, key string, request any, mutate func(id string) error) (protocol.WriteResult, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return protocol.WriteResult{}, err
	}
	digest := sha256.Sum256(payload)
	fingerprint := hex.EncodeToString(digest[:])
	var result protocol.WriteResult

	if _, err := s.query("BEGIN TRANSACTION;", nil); err != nil {
		return result, err
	}
	committed := false
	defer s.rollbackUnlessCommitted(&committed)

	rows, err := s.query("MATCH (r:Receipt {key: $key}) RETURN r.operation, r.scope, r.fingerprint, r.result_id;", map[string]any{"key": key})
	if err != nil {
		return result, err
	}
	if len(rows) == 1 {
		row := rows[0]
		if row[0] != operation || row[1] != scope || row[2] != fingerprint {
			return result, protocol.NewError("IDEMPOTENCY_CONFLICT", "This idempotency key is already in use.")
		}
		id, ok := row[3].(string)
		if !ok || id == "" {
			return result, errors.New("Invalid write receipt in the database.")
		}
		return protocol.WriteResult{ID: id, Replayed: true}, nil
	}
	result.ID = rand.Text()
	if err := mutate(result.ID); err != nil {
		return protocol.WriteResult{}, err
	}
	if _, err := s.query(`
CREATE (:Receipt {key: $key, operation: $operation, scope: $scope, fingerprint: $fingerprint, result_id: $id});`, map[string]any{
		"key": key, "operation": operation, "scope": scope, "fingerprint": fingerprint, "id": result.ID,
	}); err != nil {
		return protocol.WriteResult{}, err
	}
	if s.BeforeCommit != nil {
		s.BeforeCommit()
	}
	if _, err := s.query("COMMIT;", nil); err != nil {
		return protocol.WriteResult{}, err
	}
	committed = true
	if s.AfterCommit != nil {
		s.AfterCommit()
	}
	return result, nil
}

func (s *Store) rollbackUnlessCommitted(committed *bool) {
	if !*committed {
		_, _ = s.query("ROLLBACK;", nil)
	}
}

func (s *Store) query(statement string, parameters map[string]any) ([][]any, error) {
	var result *lbug.QueryResult
	var err error
	if len(parameters) == 0 {
		result, err = s.connection.Query(statement)
	} else {
		prepared, prepareErr := s.connection.Prepare(statement)
		if prepared != nil {
			defer prepared.Close()
		}
		if prepareErr != nil {
			return nil, prepareErr
		}
		result, err = s.connection.Execute(prepared, parameters)
	}
	if err != nil {
		return nil, err
	}
	defer result.Close()

	var rows [][]any
	for result.HasNext() {
		tuple, err := result.Next()
		if err != nil {
			return nil, err
		}
		row, err := tuple.GetAsSlice()
		tuple.Close()
		if err != nil {
			return nil, fmt.Errorf("read LadybugDB row: %w", err)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

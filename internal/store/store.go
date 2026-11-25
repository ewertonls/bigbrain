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

// Store owns one native connection. The daemon serializes access to it.
type Store struct {
	database       *lbug.Database
	connection     *lbug.Connection
	indexes        map[string]*scopeIndex
	model          *embeddings
	vectorFallback string

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
	store := &Store{database: database, connection: connection, indexes: make(map[string]*scopeIndex)}
	if err := store.openSchema(); err != nil {
		store.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() {
	s.connection.Close()
	s.database.Close()
}

func (s *Store) Record(request protocol.RecordRequest) (protocol.WriteResult, error) {
	if err := request.Validate(); err != nil {
		return protocol.WriteResult{}, err
	}
	return s.write("record", request.Scope, request.Key, "", request, func(id string) error {
		memoryType := request.Type
		if memoryType == "" {
			memoryType = "unknown"
		}
		_, err := s.query(`CREATE (:Memory {id: $id, scope: $scope, text: $text, source: $source, recorded_at: $recordedAt, memory_type: $type, status: 'active'});`, map[string]any{
			"id": id, "scope": request.Scope, "text": request.Text, "source": request.Source,
			"recordedAt": time.Now().UTC().Format(time.RFC3339Nano), "type": memoryType,
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
	return s.write("link", request.Scope, request.Key, "", request, func(id string) error {
		rows, err := s.query(`
MATCH (a:Memory {id: $from}), (b:Memory {id: $to})
WHERE (a.scope = $scope OR a.scope = 'global') AND (b.scope = $scope OR b.scope = 'global')
  AND (a.scope = $scope OR b.scope = $scope) AND a.status = 'active' AND b.status = 'active'
CREATE (a)-[:Evidence {id: $id, scope: $scope, kind: $kind, weight: $weight, source: $source}]->(b)
RETURN a.id;`, map[string]any{
			"from": request.FromID, "to": request.ToID, "scope": request.Scope, "id": id,
			"kind": request.Kind, "weight": request.Weight, "source": request.Source,
		})
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return protocol.NewError("NOT_FOUND", "The selected scope cannot link these endpoints.")
		}
		return nil
	})
}

func (s *Store) write(operation, scope, key, targetID string, request any, mutate func(id string) error) (protocol.WriteResult, error) {
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

	rows, err := s.query("MATCH (r:Receipt {key: $key}) RETURN r.operation, r.scope, r.fingerprint, r.result_id, r.deleted;", map[string]any{"key": key})
	if err != nil {
		return result, err
	}
	if len(rows) == 1 {
		row := rows[0]
		if row[4] == true && row[2] == removedFingerprint(operation, scope, fingerprint) {
			return result, protocol.NewError("RESOURCE_DELETED", "The resource created by this request has been deleted.")
		}
		if row[4] == true || row[0] != operation || row[1] != scope || row[2] != fingerprint {
			return result, protocol.NewError("IDEMPOTENCY_CONFLICT", "This idempotency key is already in use.")
		}
		id, ok := row[3].(string)
		if !ok || id == "" {
			return result, errors.New("Invalid write receipt in the database.")
		}
		return protocol.WriteResult{ID: id, Replayed: true}, nil
	}
	result.ID = targetID
	if result.ID == "" {
		result.ID = rand.Text()
	}
	if err := mutate(result.ID); err != nil {
		return protocol.WriteResult{}, err
	}
	if _, err := s.query(`
CREATE (:Receipt {key: $key, operation: $operation, scope: $scope, fingerprint: $fingerprint, result_id: $id, deleted: false});`, map[string]any{
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
	if operation != "link" {
		delete(s.indexes, scope)
	}
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

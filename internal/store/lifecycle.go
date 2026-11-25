package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

func (s *Store) Invalidate(r protocol.InvalidateRequest) (protocol.WriteResult, error) {
	if err := r.Validate(); err != nil {
		return protocol.WriteResult{}, err
	}
	return s.write("invalidate", r.Scope, r.Key, r.ID, r, func(id string) error {
		status, err := s.ownedStatus(r.Scope, id)
		if err != nil {
			return err
		}
		if status == "invalidated" {
			return protocol.NewError("ALREADY_INVALIDATED", "The memory already has a recorded invalidation.")
		}
		_, err = s.query(`MATCH (m:Memory {id:$id, scope:$scope})
SET m.status='invalidated', m.invalid_reason=$reason, m.invalid_source=$source, m.invalid_at=$at;`,
			map[string]any{"id": id, "scope": r.Scope, "reason": r.Reason, "source": r.Source, "at": time.Now().UTC().Format(time.RFC3339Nano)})
		return err
	})
}

func (s *Store) ownedStatus(scope, id string) (string, error) {
	rows, err := s.query("MATCH (m:Memory {id:$id, scope:$scope}) RETURN m.status;", map[string]any{"id": id, "scope": scope})
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", protocol.NewError("NOT_FOUND", "Memory not found in the selected scope.")
	}
	status, ok := rows[0][0].(string)
	if !ok {
		return "", errors.New("invalid memory status")
	}
	return status, nil
}

// Foundation receipts contain a hash of the canonical request. Bind the
// operation and scope to that hash before discarding their stored values.
func removedFingerprint(operation, scope, payloadHash string) string {
	sum := sha256.Sum256([]byte(operation + "\x00" + scope + "\x00" + payloadHash))
	return hex.EncodeToString(sum[:])
}

func (s *Store) Delete(r protocol.MemoryWriteRequest) (protocol.WriteResult, error) {
	if err := r.Validate(); err != nil {
		return protocol.WriteResult{}, err
	}
	return s.write("delete", r.Scope, r.Key, r.ID, r, func(id string) error {
		if _, err := s.ownedStatus(r.Scope, id); err != nil {
			return err
		}
		// A global deletion also removes incident project-owned edges. Their
		// receipts are updated here without returning their IDs or sources.
		rows, err := s.query(`
MATCH (r:Receipt) WHERE r.operation IN ['record', 'consolidate'] AND r.result_id=$id
RETURN r.key, r.operation, r.scope, r.fingerprint
UNION
MATCH (:Memory {id:$id})-[e:Evidence]-(:Memory), (r:Receipt)
WHERE r.operation='link' AND r.result_id=e.id
RETURN r.key, r.operation, r.scope, r.fingerprint;`, map[string]any{"id": id})
		if err != nil {
			return err
		}
		for _, row := range rows {
			key, okKey := row[0].(string)
			op, okOp := row[1].(string)
			scope, okScope := row[2].(string)
			hash, okHash := row[3].(string)
			if !okKey || !okOp || !okScope || !okHash {
				return errors.New("invalid creation receipt")
			}
			_, err = s.query(`MATCH (r:Receipt {key:$key})
SET r.operation=NULL, r.scope=NULL, r.result_id=NULL, r.fingerprint=$hash, r.deleted=true;`,
				map[string]any{"key": key, "hash": removedFingerprint(op, scope, hash)})
			if err != nil {
				return err
			}
		}
		_, err = s.query("MATCH (m:Memory {id:$id, scope:$scope}) DETACH DELETE m;", map[string]any{"id": id, "scope": r.Scope})
		return err
	})
}

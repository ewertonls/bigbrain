package store

import (
	"time"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

func (s *Store) Publish(r protocol.PublishRequest) (protocol.WriteResult, error) {
	if err := r.Validate(); err != nil {
		return protocol.WriteResult{}, err
	}
	return s.write("publish", r.Scope, r.Key, "", r, func(id string) error {
		rows, err := s.query(`MATCH (proposal:Memory {id: $proposal, scope: $scope, memory_type: 'lesson_proposal', status: 'active'})
CREATE (lesson:Memory {id: $id, scope: 'global', memory_type: 'lesson', status: 'active',
                      text: $text, source: $source, recorded_at: $at})
CREATE (lesson)-[:Evidence {id: CAST(gen_random_uuid() AS STRING), scope: $scope, kind: 'published_from',
                          weight: 1.0, source: $source, activation_count: 0, created_at: $at}]->(proposal)
RETURN lesson.id;`, map[string]any{
			"scope": r.Scope, "proposal": r.ProposalID, "id": id,
			"text": r.Text, "source": r.Source, "at": time.Now().UTC().Format(time.RFC3339Nano),
		})
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return protocol.NewError("NOT_FOUND", "Active lesson proposal not found in the selected project.")
		}
		return nil
	})
}

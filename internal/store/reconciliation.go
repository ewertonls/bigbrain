package store

import (
	"crypto/rand"
	"time"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

func (s *Store) Reconcile(r protocol.ReconcileRequest) (protocol.WriteResult, error) {
	if err := r.Validate(); err != nil {
		return protocol.WriteResult{}, err
	}
	return s.write("reconcile", r.Scope, r.Key, "", r, func(id string) error {
		params := map[string]any{
			"scope": r.Scope, "includeGlobal": r.IncludeGlobal, "supporting": r.SupportingID, "contrary": r.ContraryID,
			"id": id, "text": r.Text, "source": r.Source, "at": time.Now().UTC().Format(time.RFC3339Nano),
			"outcome": r.Outcome, "supersedingSide": r.SupersedingSide, "appliesWhen": r.AppliesWhen,
			"rationale": r.Rationale, "author": r.Author, "supportingLink": rand.Text(), "contraryLink": rand.Text(),
		}
		rows, err := s.query(`
MATCH (a:Memory {id:$supporting, status:'active'}), (b:Memory {id:$contrary, status:'active'})
WHERE (a.scope=$scope OR ($includeGlobal AND a.scope='global'))
  AND (b.scope=$scope OR ($includeGlobal AND b.scope='global'))
CREATE (review:Memory {id:$id, scope:$scope, text:$text, source:$source, recorded_at:$at,
  memory_type:'reconciliation', status:'active', review_outcome:$outcome, review_superseding_side:$supersedingSide,
  review_applies_when:$appliesWhen, review_rationale:$rationale, review_author:$author})
CREATE (review)-[:Evidence {id:$supportingLink, scope:$scope, kind:'supporting_claim',
  weight:1.0, source:$source, created_at:$at, activation_count:0}]->(a)
CREATE (review)-[:Evidence {id:$contraryLink, scope:$scope, kind:'contrary_claim',
  weight:1.0, source:$source, created_at:$at, activation_count:0}]->(b)
RETURN review.id;`, params)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return protocol.NewError("NOT_FOUND", "One or both claims are unavailable in the selected scope.")
		}
		return nil
	})
}

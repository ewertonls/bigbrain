package store

import (
	"errors"
	"math"
	"time"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

const visibleFeedbackEdge = `MATCH (a:Memory)-[edge:Evidence]->(b:Memory)
WHERE edge.id=$id AND (edge.scope=$scope OR edge.scope=$extraScope)
  AND (a.scope=$scope OR a.scope=$extraScope) AND (b.scope=$scope OR b.scope=$extraScope)`

// Only queries explicitly including global data can see a project's overrides
// on global edges. Native project edges and global reviews use the base values.
func edgeAttributeProjection(includeGlobal bool) (join, columns string) {
	if includeGlobal {
		return ` OPTIONAL MATCH (adj:EdgeAdjustment {id: $scope + '|' + edge.id}) `,
			`coalesce(adj.weight,edge.weight), coalesce(adj.confidence,edge.confidence),
			coalesce(adj.activation_count,edge.activation_count), coalesce(adj.last_activated_at,edge.last_activated_at,'')`
	}
	return "", `edge.weight, edge.confidence, edge.activation_count, coalesce(edge.last_activated_at,'')`
}

type feedbackState struct {
	fromID, owner, created, last string
	weight                       float64
	confidence                   *float64
	count                        *int64
}

func (s *Store) feedbackState(t protocol.FeedbackTarget) (feedbackState, error) {
	params := scopeParameters(t.Scope, t.IncludeGlobal)
	params["id"] = t.RelationshipID
	join, columns := edgeAttributeProjection(t.IncludeGlobal)
	rows, err := s.query(visibleFeedbackEdge+" AND a.status='active' AND b.status='active'"+join+
		" RETURN a.id, edge.scope, coalesce(edge.created_at,''), "+columns, params)
	if err != nil {
		return feedbackState{}, err
	}
	if len(rows) == 0 {
		return feedbackState{}, protocol.NewError("NOT_FOUND", "Active relationship not found in the selected scope.")
	}
	var st feedbackState
	row := rows[0]
	for i, dest := range []*string{&st.fromID, &st.owner, &st.created} {
		value, ok := row[i].(string)
		if !ok {
			return feedbackState{}, errors.New("invalid feedback relationship attributes")
		}
		*dest = value
	}
	var okWeight, okConfidence, okCount, okLast bool
	st.weight, okWeight = row[3].(float64)
	st.confidence, okConfidence = nullableNumber[float64](row[4])
	st.count, okCount = nullableNumber[int64](row[5])
	st.last, okLast = row[6].(string)
	if !okWeight || !okConfidence || !okCount || !okLast || len(rows) != 1 {
		return feedbackState{}, errors.New("invalid feedback relationship attributes")
	}
	return st, nil
}

func (s *Store) Feedback(r protocol.FeedbackRequest) (protocol.WriteResult, error) {
	if err := r.Validate(); err != nil {
		return protocol.WriteResult{}, err
	}
	if r.Confidence != nil && *r.Confidence == 0 {
		r.Confidence = new(float64)
	}
	return s.changeRelationship("feedback", r.FeedbackTarget, r, r.EvidenceID, "", func(st *feedbackState, now string) error {
		count := int64(0)
		if st.count != nil {
			count = *st.count
		}
		if count == math.MaxInt64 {
			return protocol.NewError("INVALID_REQUEST", "The activation count has reached its maximum.")
		}
		st.count, st.last = new(count+1), now
		st.weight++
		if r.Confidence != nil {
			st.confidence = r.Confidence
		}
		return nil
	})
}

func (s *Store) Forget(r protocol.ForgetRequest) (protocol.WriteResult, error) {
	if err := r.Validate(); err != nil {
		return protocol.WriteResult{}, err
	}
	before, _ := time.Parse(time.RFC3339Nano, r.Before)
	r.Before = before.UTC().Format(time.RFC3339Nano)
	return s.changeRelationship("forget", r.FeedbackTarget, r, "", r.Before, func(st *feedbackState, _ string) error {
		last := st.last
		if last == "" {
			last = st.created
		}
		at, err := time.Parse(time.RFC3339Nano, last)
		if err != nil || !at.Before(before) || st.weight == 0 {
			return protocol.NewError("POLICY_NOT_APPLICABLE", "Forgetting requires positive weight and a known last use or creation before the cutoff.")
		}
		st.weight /= 2
		return nil
	})
}

func (s *Store) ReviewWeight(r protocol.ReviewWeightRequest) (protocol.WriteResult, error) {
	if err := r.Validate(); err != nil {
		return protocol.WriteResult{}, err
	}
	if *r.Weight == 0 {
		r.Weight = new(float64)
	}
	return s.changeRelationship("review-weight", r.FeedbackTarget, r, "", "", func(st *feedbackState, _ string) error {
		st.weight = *r.Weight
		return nil
	})
}

func (s *Store) changeRelationship(action string, target protocol.FeedbackTarget, request any, evidence, cutoff string, apply func(*feedbackState, string) error) (protocol.WriteResult, error) {
	return s.write(action, target.Scope, target.Key, "", request, func(eventID string) error {
		st, err := s.feedbackState(target)
		if err != nil {
			return err
		}
		if evidence != "" {
			params := scopeParameters(target.Scope, target.IncludeGlobal)
			params["id"] = evidence
			rows, err := s.query("MATCH (m:Memory {id:$id,status:'active'}) WHERE "+visibleMemory+" RETURN m.id;", params)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				return protocol.NewError("NOT_FOUND", "Evidence not found in the selected scope.")
			}
		}
		// Fixed fractional precision makes chronological and lexical ordering agree.
		now := time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z")
		if err := apply(&st, now); err != nil {
			return err
		}
		params := map[string]any{
			"id": target.RelationshipID, "scope": target.Scope, "weight": st.weight, "confidence": nil, "count": nil, "last": nil,
			"event": eventID, "action": action, "at": now, "author": target.Author, "source": target.Source, "evidence": evidence, "cutoff": cutoff,
		}
		if st.confidence != nil {
			params["confidence"] = *st.confidence
		}
		if st.count != nil {
			params["count"] = *st.count
		}
		if st.last != "" {
			params["last"] = st.last
		}
		var statement string
		if st.owner == target.Scope {
			params["from"] = st.fromID
			statement = "MATCH (:Memory {id:$from})-[e:Evidence]->() WHERE e.id=$id AND e.scope=$scope "
		} else {
			params["adjustment"] = target.Scope + "|" + target.RelationshipID
			statement = "MERGE (e:EdgeAdjustment {id:$adjustment}) SET e.scope=$scope, e.relationship_id=$id "
		}
		_, err = s.query(statement+`SET e.weight=$weight, e.confidence=$confidence, e.activation_count=$count, e.last_activated_at=$last
		CREATE (:FeedbackEvent {id:$event, scope:$scope, relationship_id:$id, action:$action,
		  author:$author, source:$source, evidence_id:$evidence, recorded_at:$at, weight:$weight, confidence:$confidence,
		  activation_count:$count, last_activated_at:$last, cutoff:$cutoff});`, params)
		return err
	})
}

func (s *Store) FeedbackHistory(r protocol.FeedbackHistoryRequest) (protocol.FeedbackHistoryResult, error) {
	var result protocol.FeedbackHistoryResult
	if err := r.Validate(); err != nil {
		return result, err
	}
	params := scopeParameters(r.Scope, r.IncludeGlobal)
	params["id"] = r.RelationshipID
	rows, err := s.query(visibleFeedbackEdge+" RETURN edge.id;", params)
	if err != nil {
		return result, err
	}
	if len(rows) == 0 {
		return result, protocol.NewError("NOT_FOUND", "Relationship not found in the selected scope.")
	}
	if r.Limit == 0 {
		r.Limit = protocol.DefaultQueryLimit
	}
	const visibleEvent = "(f.scope=$scope OR f.scope=$extraScope)"
	cursorFilter := ""
	if r.After != "" {
		params["after"] = r.After
		cursor, err := s.query("MATCH (f:FeedbackEvent {id:$after,relationship_id:$id}) WHERE "+visibleEvent+" RETURN f.recorded_at;", params)
		if err != nil {
			return result, err
		}
		if len(cursor) != 1 {
			return result, protocol.NewError("INVALID_REQUEST", "Unknown cursor for this feedback history.")
		}
		params["afterTime"] = cursor[0][0]
		cursorFilter = " AND (f.recorded_at>$afterTime OR (f.recorded_at=$afterTime AND f.id>$after))"
	}
	params["limit"] = int64(r.Limit + 1)
	rows, err = s.query(`MATCH (f:FeedbackEvent {relationship_id:$id})
WHERE `+visibleEvent+cursorFilter+`
WITH f ORDER BY f.recorded_at, f.id LIMIT $limit
OPTIONAL MATCH (m:Memory {id:f.evidence_id}) WHERE m.scope=$scope OR m.scope=$extraScope
RETURN f.id, f.scope, f.action, f.author, f.source, coalesce(m.id,''), f.recorded_at, f.weight,
       f.confidence, f.activation_count, coalesce(f.last_activated_at,''), coalesce(f.cutoff,'')
ORDER BY f.recorded_at, f.id;`, params)
	if err != nil {
		return result, err
	}
	result.Events = make([]protocol.FeedbackEvent, 0, min(r.Limit, len(rows)))
	for _, row := range rows[:min(r.Limit, len(rows))] {
		event := protocol.FeedbackEvent{RelationshipID: r.RelationshipID}
		strings := []*string{&event.ID, &event.Scope, &event.Action, &event.Author, &event.Source, &event.EvidenceID, &event.RecordedAt}
		for i, dest := range strings {
			value, ok := row[i].(string)
			if !ok {
				return result, errors.New("invalid feedback event")
			}
			*dest = value
		}
		var weightOK, confOK, countOK, lastOK, cutoffOK bool
		event.Weight, weightOK = row[7].(float64)
		event.Confidence, confOK = nullableNumber[float64](row[8])
		event.ActivationCount, countOK = nullableNumber[int64](row[9])
		event.LastActivatedAt, lastOK = row[10].(string)
		event.Cutoff, cutoffOK = row[11].(string)
		if !weightOK || !confOK || !countOK || !lastOK || !cutoffOK {
			return result, errors.New("invalid feedback event attributes")
		}
		result.Events = append(result.Events, event)
	}
	if len(rows) > r.Limit {
		result.NextCursor = result.Events[r.Limit-1].ID
	}
	return result, nil
}

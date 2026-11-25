package store

import (
	"cmp"
	"errors"
	"slices"
	"strconv"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

func (s *Store) History(r protocol.HistoryRequest) (protocol.HistoryResult, error) {
	var history protocol.HistoryResult
	if err := r.Validate(); err != nil {
		return history, err
	}
	params := scopeParameters(r.Scope, r.IncludeGlobal)
	params["id"] = r.ID
	rows, err := s.query("MATCH (m:Memory {id:$id}) WHERE "+visibleMemory+" RETURN "+memoryColumns+
		", m.status, m.invalid_reason, m.invalid_source, m.invalid_at;", params)
	if err != nil {
		return history, err
	}
	if len(rows) == 0 {
		return history, protocol.NewError("NOT_FOUND", "Memory not found in the selected scope.")
	}
	row := rows[0]
	history.Memory, err = decodeMemory(row)
	if err != nil {
		return history, err
	}
	history.Status, _ = row[6].(string)
	switch history.Status {
	case "active":
	case "invalidated":
		reason, okReason := row[7].(string)
		source, okSource := row[8].(string)
		at, okAt := row[9].(string)
		if !okReason || !okSource || !okAt {
			return history, errors.New("invalid memory history")
		}
		history.Invalidation = &protocol.Invalidation{Reason: reason, Source: source, At: at}
	default:
		return history, errors.New("invalid memory status")
	}
	if r.Limit == 0 {
		r.Limit = protocol.DefaultQueryLimit
	}
	edges, err := s.visibleEdges(r, false)
	if err != nil {
		return history, err
	}
	slices.SortFunc(edges, func(a, b protocol.RelatedMemory) int {
		return cmp.Compare(a.Path.RelationshipID, b.Path.RelationshipID)
	})
	history.Related = edges[:min(len(edges), r.Limit)]
	if len(edges) > r.Limit {
		history.NextCursor = history.Related[r.Limit-1].Path.RelationshipID
	}
	return history, nil
}

func (s *Store) visibleEdges(r protocol.HistoryRequest, activeOnly bool) ([]protocol.RelatedMemory, error) {
	params := scopeParameters(r.Scope, r.IncludeGlobal)
	params["id"] = r.ID
	filter := ""
	end := ""
	if activeOnly {
		filter = " AND seed.status='active' AND neighbor.status='active'"
	} else {
		params["after"] = r.After
		filter = " AND edge.id > $after"
		end = " ORDER BY edge.id LIMIT " + strconv.Itoa(r.Limit+1)
	}
	edges := make([]protocol.RelatedMemory, 0)
	for _, walk := range []struct{ pattern, direction string }{{"-[edge:Evidence]->", "forward"}, {"<-[edge:Evidence]-", "reverse"}} {
		rows, err := s.query("MATCH (seed:Memory {id:$id})"+walk.pattern+`(neighbor:Memory)
WHERE (seed.scope=$scope OR seed.scope=$extraScope) AND (edge.scope=$scope OR edge.scope=$extraScope)
  AND (neighbor.scope=$scope OR neighbor.scope=$extraScope)`+filter+`
RETURN neighbor.id, neighbor.scope, neighbor.memory_type, neighbor.status, neighbor.source,
       edge.id, edge.scope, edge.kind, edge.source, edge.weight,
       edge.confidence, edge.activation_count, coalesce(edge.last_activated_at, ''), coalesce(edge.created_at, '')`+end, params)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			var field [9]string
			for i := range field {
				value, ok := row[i].(string)
				if !ok {
					return nil, errors.New("invalid related memory or relationship")
				}
				field[i] = value
			}
			weight, okWeight := row[9].(float64)
			confidence, okConfidence := nullableNumber[float64](row[10])
			activations, okActivations := nullableNumber[int64](row[11])
			lastAt, okLastAt := row[12].(string)
			createdAt, okCreatedAt := row[13].(string)
			if !okWeight || !okConfidence || !okActivations || !okLastAt || !okCreatedAt {
				return nil, errors.New("invalid relationship attributes")
			}
			from, to := r.ID, field[0]
			if walk.direction == "reverse" {
				// The forward row already represents this self-link.
				if from == to {
					continue
				}
				from, to = to, from
			}
			edges = append(edges, protocol.RelatedMemory{
				ID: field[0], Scope: field[1], Type: field[2], Status: field[3], Source: field[4],
				Path: protocol.Path{
					RelationshipID: field[5], Scope: field[6], Kind: field[7], Source: field[8],
					FromID: from, ToID: to, Direction: walk.direction, Weight: weight, Confidence: confidence,
					ActivationCount: activations, LastActivatedAt: lastAt, CreatedAt: createdAt,
				},
			})
		}
	}
	return edges, nil
}

func nullableNumber[T float64 | int64](value any) (*T, bool) {
	if value == nil {
		return nil, true
	}
	number, ok := value.(T)
	return &number, ok
}

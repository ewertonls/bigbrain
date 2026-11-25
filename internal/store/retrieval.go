package store

import (
	"cmp"
	"errors"
	"slices"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

const neighborsPerMatch = 5

func (s *Store) Query(request protocol.QueryRequest) (protocol.QueryResult, error) {
	if err := request.Validate(); err != nil {
		return protocol.QueryResult{}, err
	}
	if request.ID != "" {
		memories, err := s.visibleMemories(request)
		if err != nil {
			return protocol.QueryResult{}, err
		}
		matches := make([]protocol.Match, 0, len(memories))
		for _, m := range memories {
			matches = append(matches, protocol.Match{Memory: m, Reason: protocol.Reason{Kind: "id"}})
		}
		return protocol.QueryResult{Matches: matches, Count: len(matches)}, nil
	}
	indexes, err := s.visibleIndexes(request)
	if err != nil {
		return protocol.QueryResult{}, err
	}
	limit := request.Limit
	if limit == 0 {
		limit = protocol.DefaultQueryLimit
	}
	matches := indexes.textMatches(request.Text)
	var fallback string
	if request.Mode == "hybrid" {
		var vector []float32
		vector, fallback = s.queryVector(request.Text, indexes)
		if fallback == "" {
			matches, err = s.hybrid(request, indexes, matches, vector, limit)
			if err != nil {
				return protocol.QueryResult{}, err
			}
		}
	}
	if request.Mode == "" || request.Mode == "associative" || fallback != "" {
		indirect, err := s.associations(request, matches, indexes)
		if err != nil {
			return protocol.QueryResult{}, err
		}
		matches = append(matches, indirect...)
	}
	return protocol.QueryResult{Matches: matches[:min(limit, len(matches))], Count: len(matches), Fallback: fallback}, nil
}

func (s *Store) visibleMemories(request protocol.QueryRequest) ([]protocol.Memory, error) {
	parameters := scopeParameters(request.Scope, request.IncludeGlobal)
	statement := "MATCH (m:Memory) WHERE " + visibleMemory + " AND m.status = 'active'"
	if request.ID != "" {
		statement += " AND m.id = $id"
		parameters["id"] = request.ID
	}
	rows, err := s.query(statement+" RETURN "+memoryColumns, parameters)
	if err != nil {
		return nil, err
	}
	memories := make([]protocol.Memory, 0, len(rows))
	for _, row := range rows {
		m, err := decodeMemory(row)
		if err != nil {
			return nil, err
		}
		memories = append(memories, m)
	}
	return memories, nil
}

const visibleMemory = "(m.scope = $scope OR m.scope = $extraScope)"
const memoryColumns = "m.id, m.scope, m.text, m.source, m.recorded_at, m.memory_type"

func decodeMemory(row []any) (protocol.Memory, error) {
	var fields [6]string
	for i := range fields {
		v, ok := row[i].(string)
		if !ok {
			return protocol.Memory{}, errors.New("Invalid memory in the database.")
		}
		fields[i] = v
	}
	return protocol.Memory{ID: fields[0], Scope: fields[1], Text: fields[2], Source: fields[3], RecordedAt: fields[4], Type: fields[5]}, nil
}

func scopeParameters(scope string, includeGlobal bool) map[string]any {
	extraScope := scope
	if includeGlobal {
		extraScope = "global"
	}
	return map[string]any{"scope": scope, "extraScope": extraScope}
}

func (s *Store) associations(request protocol.QueryRequest, seeds []protocol.Match, indexes collection) ([]protocol.Match, error) {
	direct := make(map[string]bool, len(seeds))
	for _, seed := range seeds {
		direct[seed.ID] = true
	}
	bestByID := make(map[string]protocol.Match)
	for _, seed := range seeds {
		neighbors, err := s.neighbors(request, seed.Memory, indexes)
		if err != nil {
			return nil, err
		}
		for _, neighbor := range neighbors {
			if !direct[neighbor.ID] {
				keepBest(bestByID, neighbor)
			}
		}
	}
	return sortedNeighbors(bestByID), nil
}

func (s *Store) neighbors(request protocol.QueryRequest, seed protocol.Memory, indexes collection) ([]protocol.Match, error) {
	bestByID := make(map[string]protocol.Match)
	edges, err := s.visibleEdges(protocol.HistoryRequest{Scope: request.Scope, IncludeGlobal: request.IncludeGlobal, ID: seed.ID}, true)
	if err != nil {
		return nil, err
	}
	for _, edge := range edges {
		neighbor := indexes.document(edge.ID)
		if neighbor == nil || neighbor.ID == seed.ID {
			continue
		}
		keepBest(bestByID, protocol.Match{Memory: neighbor.Memory, Reason: protocol.Reason{Kind: "relationship", Path: &edge.Path}})
	}
	neighbors := sortedNeighbors(bestByID)
	return neighbors[:min(neighborsPerMatch, len(neighbors))], nil
}

func keepBest(bestByID map[string]protocol.Match, match protocol.Match) {
	current, exists := bestByID[match.ID]
	if !exists || compareNeighbors(match, current) < 0 {
		bestByID[match.ID] = match
	}
}

func sortedNeighbors(byID map[string]protocol.Match) []protocol.Match {
	matches := make([]protocol.Match, 0, len(byID))
	for _, match := range byID {
		matches = append(matches, match)
	}
	slices.SortFunc(matches, compareNeighbors)
	return matches
}

func compareNeighbors(a, b protocol.Match) int {
	if weightOrder := cmp.Compare(b.Reason.Path.Weight, a.Reason.Path.Weight); weightOrder != 0 {
		return weightOrder
	}
	if idOrder := cmp.Compare(a.ID, b.ID); idOrder != 0 {
		return idOrder
	}
	return cmp.Compare(a.Reason.Path.RelationshipID, b.Reason.Path.RelationshipID)
}

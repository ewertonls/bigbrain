package store

import (
	"cmp"
	"errors"
	"slices"
	"strings"
	"unicode"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

const neighborsPerMatch = 5

func (s *Store) Query(request protocol.QueryRequest) (protocol.QueryResult, error) {
	if err := request.Validate(); err != nil {
		return protocol.QueryResult{}, err
	}
	memories, err := s.visibleMemories(request)
	if err != nil {
		return protocol.QueryResult{}, err
	}
	matches := make([]protocol.Match, 0)
	if request.ID != "" {
		if len(memories) != 0 {
			matches = append(matches, protocol.Match{Memory: memories[0], Reason: protocol.Reason{Kind: "id"}})
		}
	} else {
		matches = textMatches(memories, request.Text)
		if request.Mode != "text" && len(matches) != 0 {
			indirect, err := s.associations(matches, memories)
			if err != nil {
				return protocol.QueryResult{}, err
			}
			matches = append(matches, indirect...)
		}
	}
	limit := request.Limit
	if limit == 0 {
		limit = protocol.DefaultQueryLimit
	}
	return protocol.QueryResult{Matches: matches[:min(limit, len(matches))], Count: len(matches)}, nil
}

func (s *Store) visibleMemories(request protocol.QueryRequest) ([]protocol.Memory, error) {
	extraScope := request.Scope
	if request.IncludeGlobal {
		extraScope = "global"
	}
	statement := "MATCH (m:Memory) WHERE (m.scope = $scope OR m.scope = $extraScope)"
	parameters := map[string]any{"scope": request.Scope, "extraScope": extraScope}
	if request.ID != "" {
		statement += " AND m.id = $id"
		parameters["id"] = request.ID
	}
	rows, err := s.query(statement+" RETURN m.id, m.scope, m.text, m.source, m.recorded_at, m.memory_type;", parameters)
	if err != nil {
		return nil, err
	}
	memories := make([]protocol.Memory, 0, len(rows))
	for _, row := range rows {
		var fields [6]string
		for index, value := range row {
			text, ok := value.(string)
			if !ok {
				return nil, errors.New("Invalid memory in the database.")
			}
			fields[index] = text
		}
		memories = append(memories, protocol.Memory{
			ID: fields[0], Scope: fields[1], Text: fields[2], Source: fields[3], RecordedAt: fields[4], Type: fields[5],
		})
	}
	return memories, nil
}

func words(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}

func textMatches(memories []protocol.Memory, text string) []protocol.Match {
	terms := words(text)
	slices.Sort(terms)
	terms = slices.Compact(terms)
	matches := make([]protocol.Match, 0)
	for _, memory := range memories {
		wordsInMemory := make(map[string]bool)
		for _, word := range words(memory.Text) {
			wordsInMemory[word] = true
		}
		var found []string
		for _, term := range terms {
			if wordsInMemory[term] {
				found = append(found, term)
			}
		}
		if len(found) > 0 {
			matches = append(matches, protocol.Match{
				Memory: memory,
				Reason: protocol.Reason{Kind: "text", Terms: found},
			})
		}
	}
	slices.SortFunc(matches, func(a, b protocol.Match) int {
		if scoreOrder := cmp.Compare(len(b.Reason.Terms), len(a.Reason.Terms)); scoreOrder != 0 {
			return scoreOrder
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return matches
}

func (s *Store) associations(seeds []protocol.Match, memories []protocol.Memory) ([]protocol.Match, error) {
	visible := make(map[string]protocol.Memory, len(memories))
	for _, memory := range memories {
		visible[memory.ID] = memory
	}
	direct := make(map[string]bool, len(seeds))
	for _, seed := range seeds {
		direct[seed.ID] = true
	}
	bestByID := make(map[string]protocol.Match)
	for _, seed := range seeds {
		neighbors, err := s.neighbors(seed.Memory, visible)
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

func (s *Store) neighbors(seed protocol.Memory, visible map[string]protocol.Memory) ([]protocol.Match, error) {
	bestByID := make(map[string]protocol.Match)
	for _, traversal := range []struct {
		relation  string
		direction string
	}{
		{"-[edge:Evidence]->", "forward"},
		{"<-[edge:Evidence]-", "reverse"},
	} {
		rows, err := s.query(`
MATCH (seed:Memory {id: $id})`+traversal.relation+`(neighbor:Memory)
WHERE seed.scope = $scope AND neighbor.scope = $scope
RETURN neighbor.id, edge.id, edge.kind, edge.source, edge.weight;`,
			map[string]any{"id": seed.ID, "scope": seed.Scope})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			neighborID, idOK := row[0].(string)
			relID, relOK := row[1].(string)
			kind, kindOK := row[2].(string)
			source, sourceOK := row[3].(string)
			weight, weightOK := row[4].(float64)
			if !idOK || !relOK || !kindOK || !sourceOK || !weightOK {
				return nil, errors.New("Invalid relationship in the database.")
			}
			neighbor, ok := visible[neighborID]
			if !ok || neighbor.ID == seed.ID {
				continue
			}
			fromID, toID := seed.ID, neighbor.ID
			if traversal.direction == "reverse" {
				fromID, toID = toID, fromID
			}
			keepBest(bestByID, protocol.Match{Memory: neighbor, Reason: protocol.Reason{
				Kind: "relationship",
				Path: &protocol.Path{
					RelationshipID: relID, FromID: fromID, ToID: toID, Kind: kind, Source: source,
					Direction: traversal.direction, Weight: weight,
				},
			}})
		}
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

package store

import (
	"cmp"
	"log"
	"os"
	"slices"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

func (s *Store) vectorModel() (*embeddings, string) {
	if s.model != nil || s.vectorFallback != "" {
		return s.model, s.vectorFallback
	}
	path := os.Getenv("BIGBRAIN_EMBEDDINGS")
	switch {
	case os.Getenv("BIGBRAIN_VECTOR_INDEX") == "off":
		s.vectorFallback = "vector_index_disabled"
	case path == "":
		s.vectorFallback = "model_not_configured"
	default:
		model, err := loadEmbeddings(path)
		if err != nil {
			log.Printf("Could not load local embeddings: %v", err)
			s.vectorFallback = "model_unavailable"
		} else {
			s.model = model
		}
	}
	return s.model, s.vectorFallback
}

func (s *Store) queryVector(text string, indexes collection) ([]float32, string) {
	model, fallback := s.vectorModel()
	if model == nil {
		return nil, fallback
	}
	vector := model.encode(text)
	if len(vector) == 0 {
		return nil, "no_query_embedding"
	}
	if !indexes.indexVectors(model) {
		return nil, "no_visible_embeddings"
	}
	return vector, ""
}

// The graph gets one vote per target from its strongest visible path. Freeze
// the seed ranking before traversal so new neighbors cannot propagate again.
func (s *Store) hybrid(request protocol.QueryRequest, indexes collection, lexical []protocol.Match, vector []float32, limit int) ([]protocol.Match, error) {
	pool := max(limit, neighborsPerMatch)
	byID := make(map[string]protocol.Match, len(lexical))
	for _, m := range lexical {
		m.Reason.Score = m.Reason.BM25 / (1 + m.Reason.BM25)
		byID[m.ID] = m
	}
	for _, v := range indexes.nearest(vector, pool) {
		if v.Reason.Cosine < 0.45 {
			break
		}
		m, ok := byID[v.ID]
		if !ok {
			m = v
		} else {
			m.Reason.Kind = "hybrid"
			m.Reason.Cosine = v.Reason.Cosine
		}
		m.Reason.Score += 0.65 * v.Reason.Cosine
		byID[m.ID] = m
	}
	seeds := rankedMatches(byID)
	for _, seed := range seeds[:min(pool, len(seeds))] {
		neighbors, err := s.neighbors(request, seed.Memory, indexes)
		if err != nil {
			return nil, err
		}
		for _, neighbor := range neighbors {
			path := neighbor.Reason.Path
			contribution := seed.Reason.Score * 0.8 * (path.Weight / (1 + path.Weight))
			if contribution <= 0 {
				continue
			}
			m, exists := byID[neighbor.ID]
			if !exists {
				m = neighbor
			}
			if m.Reason.GraphScore > contribution ||
				m.Reason.GraphScore == contribution && m.Reason.Path.RelationshipID <= path.RelationshipID {
				continue
			}
			m.Reason.Path = path
			m.Reason.GraphScore = contribution
			if m.Reason.Kind != "relationship" {
				m.Reason.Kind = "hybrid"
			}
			byID[neighbor.ID] = m
		}
	}
	for id, m := range byID {
		m.Reason.Score += m.Reason.GraphScore
		byID[id] = m
	}
	return rankedMatches(byID), nil
}

func rankedMatches(byID map[string]protocol.Match) []protocol.Match {
	matches := make([]protocol.Match, 0, len(byID))
	for _, m := range byID {
		matches = append(matches, m)
	}
	slices.SortFunc(matches, func(a, b protocol.Match) int {
		if order := cmp.Compare(b.Reason.Score, a.Reason.Score); order != 0 {
			return order
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return matches
}

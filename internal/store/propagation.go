package store

import (
	"cmp"
	"maps"
	"slices"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

type graphNeighbor struct {
	memory   *indexedMemory
	path     protocol.Path
	priority float64
}

// Express the confidence-adjusted strength as a weight. Normalizing this
// value with p/(1+p) yields w/(1+w) * factor. In particular, unknown or
// full confidence preserves the exact Foundation weight for neighbor ordering.
func linkPriority(path protocol.Path) float64 {
	if path.Confidence == nil {
		return path.Weight
	}
	factor := (1 + *path.Confidence) / 2
	return path.Weight * factor / (1 + path.Weight*(1-factor))
}

type graphHit struct {
	memory *indexedMemory
	score  float64
	hops   int
	steps  [protocol.MaxGraphDepth]*protocol.Path
}

type walkPath struct {
	visited   []string
	steps     [protocol.MaxGraphDepth]*protocol.Path
	strengths [protocol.MaxGraphDepth]float64
}

func (path walkPath) score(seed float64, hops int) float64 {
	// Multiply in a consistent order so routes with the same factors tie
	// even when they encounter the relationships in a different order.
	factors := path.strengths[:hops]
	slices.Sort(factors)
	for _, factor := range factors {
		seed = seed * 0.8 * factor
	}
	return seed
}

func (hit graphHit) betterThan(other graphHit) bool {
	if hit.score != other.score {
		return hit.score > other.score
	}
	if hit.hops != other.hops {
		return hit.hops < other.hops
	}
	return slices.CompareFunc(hit.steps[:hit.hops], other.steps[:other.hops], func(a, b *protocol.Path) int {
		return cmp.Compare(a.RelationshipID, b.RelationshipID)
	}) < 0
}

func (hit graphHit) explain(reason *protocol.Reason) {
	reason.GraphScore = hit.score
	reason.Path = hit.steps[hit.hops-1]
	if hit.hops > 1 {
		reason.Hops = hit.hops
		reason.Steps = make([]protocol.Path, hit.hops)
		for i, step := range hit.steps[:hit.hops] {
			reason.Steps[i] = *step
		}
	}
}

type propagation struct {
	store        *Store
	request      protocol.QueryRequest
	indexes      collection
	maxDepth     int
	maxNeighbors int
	hybrid       bool
	neighbors    map[string][]graphNeighbor
	best         map[string]graphHit
}

func (s *Store) propagate(request protocol.QueryRequest, indexes collection, seeds []protocol.Match, hybrid bool) (map[string]graphHit, error) {
	g := propagation{
		store: s, request: request, indexes: indexes, hybrid: hybrid,
		maxDepth: protocol.DefaultMaxDepth, maxNeighbors: protocol.DefaultMaxNeighbors,
		neighbors: make(map[string][]graphNeighbor), best: make(map[string]graphHit),
	}
	if request.MaxDepth != nil {
		g.maxDepth = *request.MaxDepth
	}
	if request.MaxNeighbors != nil {
		g.maxNeighbors = *request.MaxNeighbors
	}
	for _, seed := range seeds {
		score := seed.Reason.Score
		if !hybrid {
			score = seed.Reason.BM25 / (1 + seed.Reason.BM25)
		}
		if err := g.walk(walkPath{visited: []string{seed.ID}}, score); err != nil {
			return nil, err
		}
	}
	return g.best, nil
}

func (g *propagation) walk(path walkPath, seedScore float64) error {
	hops := len(path.visited)
	neighbors, err := g.neighborsOf(path.visited[hops-1])
	if err != nil {
		return err
	}
	taken := 0
	for i := range neighbors {
		n := &neighbors[i]
		if slices.Contains(path.visited, n.memory.ID) {
			continue
		}
		path.steps[hops-1] = &n.path
		path.strengths[hops-1] = n.priority / (1 + n.priority)
		score := path.score(seedScore, hops)
		if g.hybrid && score == 0 {
			break // Neighbors are ordered by decreasing priority.
		}
		if !g.hybrid && g.maxDepth == 1 {
			score = n.priority // Foundation orders indirect matches by link priority alone.
		}
		hit := graphHit{memory: n.memory, score: score, hops: hops, steps: path.steps}
		if old, found := g.best[n.memory.ID]; !found || hit.betterThan(old) {
			g.best[n.memory.ID] = hit
		}
		// A weaker route can still reach a different neighbor because its visited nodes differ.
		if hops < g.maxDepth {
			next := path
			next.visited = append(path.visited, n.memory.ID)
			if err := g.walk(next, seedScore); err != nil {
				return err
			}
		}
		taken++
		if taken == g.maxNeighbors {
			break
		}
	}
	return nil
}

func (g *propagation) neighborsOf(id string) ([]graphNeighbor, error) {
	if neighbors, found := g.neighbors[id]; found {
		return neighbors, nil
	}
	edges, err := g.store.visibleEdges(protocol.HistoryRequest{Scope: g.request.Scope, IncludeGlobal: g.request.IncludeGlobal, ID: id}, true)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]graphNeighbor)
	for _, edge := range edges {
		memory := g.indexes.document(edge.ID)
		if edge.ID == id || memory == nil {
			continue
		}
		n := graphNeighbor{memory: memory, path: edge.Path, priority: linkPriority(edge.Path)}
		old, exists := byID[edge.ID]
		if !exists || n.priority > old.priority || n.priority == old.priority && n.path.RelationshipID < old.path.RelationshipID {
			byID[edge.ID] = n
		}
	}
	neighbors := slices.Collect(maps.Values(byID))
	slices.SortFunc(neighbors, func(a, b graphNeighbor) int {
		if order := cmp.Compare(b.priority, a.priority); order != 0 {
			return order
		}
		return cmp.Compare(a.memory.ID, b.memory.ID)
	})
	// At most maxDepth-1 other nodes on a path can be ineligible on a later visit.
	keep := g.maxNeighbors + g.maxDepth - 1
	if len(neighbors) > keep {
		neighbors = slices.Clone(neighbors[:keep])
	}
	g.neighbors[id] = neighbors
	return neighbors, nil
}

func (s *Store) associations(request protocol.QueryRequest, seeds []protocol.Match, indexes collection) ([]protocol.Match, error) {
	hits, err := s.propagate(request, indexes, seeds, false)
	if err != nil {
		return nil, err
	}
	for _, seed := range seeds {
		delete(hits, seed.ID)
	}
	ordered := slices.Collect(maps.Values(hits))
	slices.SortFunc(ordered, func(a, b graphHit) int {
		if order := cmp.Compare(b.score, a.score); order != 0 {
			return order
		}
		return cmp.Compare(a.memory.ID, b.memory.ID)
	})
	matches := make([]protocol.Match, 0, len(ordered))
	for _, hit := range ordered {
		reason := protocol.Reason{Kind: "relationship"}
		hit.explain(&reason)
		if request.MaxDepth == nil || *request.MaxDepth == 1 {
			reason.GraphScore = 0 // Keep the one-hop associative response shape.
		}
		matches = append(matches, protocol.Match{Memory: hit.memory.Memory, Reason: reason})
	}
	return matches, nil
}

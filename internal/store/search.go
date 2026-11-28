package store

import (
	"cmp"
	"math"
	"slices"
	"strings"
	"unicode"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

type indexedMemory struct {
	protocol.Memory
	length int
	vector []float32
}

type scopeIndex struct {
	docs         map[string]*indexedMemory
	postings     map[string]map[string]int
	totalLength  int
	scope        string
	indexedBy    *encoder
	embeddedDocs int
}

// A request searches one scope or the union of a project and global.
type collection []*scopeIndex

func (s *Store) visibleIndexes(request protocol.QueryRequest) (collection, error) {
	scopes := []string{request.Scope}
	if request.IncludeGlobal {
		scopes = append(scopes, "global")
	}
	var visible collection
	for _, scope := range scopes {
		if scope != "global" && scope != s.projectIndex {
			delete(s.indexes, s.projectIndex)
			s.projectIndex = scope
		}
		idx := s.indexes[scope]
		if idx == nil {
			memories, err := s.visibleMemories(protocol.QueryRequest{Scope: scope})
			if err != nil {
				return nil, err
			}
			idx = buildIndex(memories)
			idx.scope = scope
			if len(memories) != 0 {
				s.indexes[scope] = idx
			}
		}
		visible = append(visible, idx)
	}
	return visible, nil
}

func words(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}

func buildIndex(memories []protocol.Memory) *scopeIndex {
	idx := &scopeIndex{docs: make(map[string]*indexedMemory, len(memories)), postings: make(map[string]map[string]int)}
	for _, m := range memories {
		tokens := words(strings.ToLower(m.Text))
		idx.docs[m.ID] = &indexedMemory{Memory: m, length: len(tokens)}
		idx.totalLength += len(tokens)
		for _, term := range tokens {
			if idx.postings[term] == nil {
				idx.postings[term] = make(map[string]int)
			}
			idx.postings[term][m.ID]++
		}
	}
	return idx
}

func (c collection) document(id string) *indexedMemory {
	for _, scope := range c {
		if doc := scope.docs[id]; doc != nil {
			return doc
		}
	}
	return nil
}

func (c collection) textMatches(text string) []protocol.Match {
	const k1, b = 1.2, 0.75
	var documents, totalLength int
	for _, idx := range c {
		documents += len(idx.docs)
		totalLength += idx.totalLength
	}
	matches := make([]protocol.Match, 0)
	if totalLength == 0 {
		return matches
	}
	avgLength := float64(totalLength) / float64(documents)
	terms := words(strings.ToLower(text))
	slices.Sort(terms)
	positions := make(map[string]int)
	for _, term := range slices.Compact(terms) {
		df := 0
		for _, idx := range c {
			df += len(idx.postings[term])
		}
		idf := math.Log1p((float64(documents-df) + 0.5) / (float64(df) + 0.5))
		for _, idx := range c {
			for id, freq := range idx.postings[term] {
				doc := idx.docs[id]
				pos, found := positions[id]
				if !found {
					pos = len(matches)
					positions[id] = pos
					matches = append(matches, protocol.Match{Memory: doc.Memory, Reason: protocol.Reason{Kind: "text"}})
				}
				reason := &matches[pos].Reason
				reason.Terms = append(reason.Terms, term)
				tf := float64(freq)
				reason.BM25 += idf * tf * (k1 + 1) / (tf + k1*(1-b+b*float64(doc.length)/avgLength))
			}
		}
	}
	// Foundation controls order by the number of distinct matched terms.
	slices.SortFunc(matches, func(a, b protocol.Match) int {
		if order := cmp.Compare(len(b.Reason.Terms), len(a.Reason.Terms)); order != 0 {
			return order
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return matches
}

// All indexed vectors have unit length. Keep the best k while scanning visible scopes.
func (c collection) nearest(query []float32, k int) []protocol.Match {
	if len(query) == 0 || k <= 0 {
		return nil
	}
	// One spare slot lets an insertion displace the previous last match.
	matches := make([]protocol.Match, 0, k+1)
	for _, idx := range c {
		for _, doc := range idx.docs {
			if len(doc.vector) == 0 {
				continue
			}
			var dot float64
			for i, v := range query {
				dot += float64(v) * float64(doc.vector[i])
			}
			cosine := max(-1, min(1, dot))
			if len(matches) == k && cosine < matches[k-1].Reason.Cosine {
				continue
			}
			match := protocol.Match{Memory: doc.Memory, Reason: protocol.Reason{Kind: "vector", Cosine: cosine}}
			pos, _ := slices.BinarySearchFunc(matches, match, compareVectorMatches)
			if pos < k {
				matches = slices.Insert(matches, pos, match)
				matches = matches[:min(k, len(matches))]
			}
		}
	}
	return matches
}

func compareVectorMatches(a, b protocol.Match) int {
	if order := cmp.Compare(b.Reason.Cosine, a.Reason.Cosine); order != 0 {
		return order
	}
	return cmp.Compare(a.ID, b.ID)
}

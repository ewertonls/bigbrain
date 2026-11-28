package store

import (
	"cmp"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"

	"github.com/ewertonls/bigbrain/internal/protocol"
	"golang.org/x/sys/unix"
)

// Check file identity between queries. A query holds one immutable set of
// weights and tokenizer rules for both its documents and its search text.
type modelFile struct {
	device, inode uint64
	size          int64
	mtime, ctime  unix.Timespec
	directory     bool
}

func statModelFile(path string) (modelFile, error) {
	var info unix.Stat_t
	if err := unix.Stat(path, &info); err != nil {
		return modelFile{}, &os.PathError{Op: "stat", Path: path, Err: err}
	}
	return modelFile{
		device: uint64(info.Dev), inode: uint64(info.Ino), size: info.Size,
		mtime: info.Mtim, ctime: info.Ctim, directory: info.Mode&unix.S_IFMT == unix.S_IFDIR,
	}, nil
}

func modelFiles(path string) ([]modelFile, error) {
	info, err := statModelFile(path)
	if err != nil {
		return nil, err
	}
	if !info.directory {
		return []modelFile{info}, nil
	}
	var files []modelFile
	for _, name := range []string{"config.json", "tokenizer.json", "model.safetensors"} {
		f, err := statModelFile(filepath.Join(path, name))
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}

func (s *Store) vectorModel() (*encoder, string) {
	path := os.Getenv("BIGBRAIN_EMBEDDINGS")
	if os.Getenv("BIGBRAIN_VECTOR_INDEX") == "off" {
		return nil, "vector_index_disabled"
	}
	if path == "" {
		return nil, "model_not_configured"
	}
	files, err := modelFiles(path)
	if err == nil && slices.Equal(s.modelFiles, files) {
		return s.model, s.vectorFallback
	}
	var model *encoder
	if err == nil {
		model, err = loadEmbeddings(path)
		if err == nil {
			var after []modelFile
			after, err = modelFiles(path)
			if err == nil && !slices.Equal(files, after) {
				err = fmt.Errorf("model files changed during loading")
				files = nil // Retry with a coherent snapshot on the next query.
			}
		}
	}
	// Release references to the old encoder even in scopes not searched now.
	for _, idx := range s.indexes {
		idx.indexedBy = nil
	}
	s.model, s.modelFiles, s.vectorFallback = model, files, ""
	if err != nil {
		s.model, s.vectorFallback = nil, "model_unavailable"
		if s.modelError != err.Error() {
			log.Printf("Cannot load BIGBRAIN_EMBEDDINGS %q: %v. Check the local model files and their checksums.", path, err)
		}
		s.modelError = err.Error()
	} else {
		s.modelError = ""
	}
	return s.model, s.vectorFallback
}

func (s *Store) queryVector(ctx context.Context, text string, indexes collection, progress func(protocol.IndexProgress)) ([]float32, string, error) {
	model, fallback := s.vectorModel()
	if model == nil {
		return nil, fallback, nil
	}
	vector := model.encode(text)
	if len(vector) == 0 {
		return nil, "no_query_embedding", nil
	}
	count := 0
	for _, idx := range indexes {
		if err := s.indexVectors(ctx, idx, model, progress); err != nil {
			if ctx.Err() != nil {
				return nil, "", ctx.Err()
			}
			log.Printf("Cannot use embedding cache %s: %v. Stop BigBrain and move the cache aside to rebuild.", s.cachePath, err)
			return nil, "vector_cache_unavailable", nil
		}
		count += idx.embeddedDocs
	}
	if count == 0 {
		return nil, "no_visible_embeddings", nil
	}
	return vector, "", nil
}

// Seed scores are frozen before adding a single graph contribution per target.
func (s *Store) hybrid(request protocol.QueryRequest, indexes collection, lexical []protocol.Match, vector []float32, limit int) ([]protocol.Match, error) {
	pool := max(limit, protocol.DefaultMaxNeighbors)
	byID := make(map[string]protocol.Match, len(lexical))
	for _, m := range lexical {
		m.Reason.Score = m.Reason.BM25 / (1 + m.Reason.BM25)
		byID[m.ID] = m
	}
	for _, v := range indexes.nearest(vector, pool) {
		if v.Reason.Cosine < s.model.minCosine {
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
	hits, err := s.propagate(request, indexes, seeds[:min(pool, len(seeds))], true)
	if err != nil {
		return nil, err
	}
	for id, hit := range hits {
		match, direct := byID[id]
		if direct {
			match.Reason.Kind = "hybrid"
		} else {
			match.Memory = hit.memory.Memory
			match.Reason.Kind = "relationship"
		}
		hit.explain(&match.Reason)
		match.Reason.Score += hit.score
		byID[id] = match
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

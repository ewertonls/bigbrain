package store

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/ewertonls/bigbrain/internal/blink"
)

// Development value pending the pinned release sweep in TestBlinkCutoffCalibration.
const blinkMinCosine = 0.20

type encoder struct {
	encode      func(string) []float32
	dim         int
	fingerprint [sha256.Size]byte
	minCosine   float64
}

func loadEmbeddings(path string) (*encoder, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		model, err := blink.Load(path)
		if err != nil {
			return nil, err
		}
		encode := func(text string) []float32 {
			vector, ok := model.Encode(text)
			if !ok {
				return nil
			}
			return vector[:]
		}
		return &encoder{encode, blink.Dimension, model.Fingerprint, blinkMinCosine}, nil
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: expected a Blink bundle directory or a regular word2vec file", path)
	}
	return loadWordVectors(path)
}

// Read local word2vec text vectors, optionally gzip-compressed. The first
// line gives the vocabulary size and dimensions. No network access is needed.
type embeddings struct {
	dim   int
	words map[string][]float32
}

func loadWordVectors(path string) (*encoder, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	hash := sha256.New()
	io.WriteString(hash, "bigbrain-word2vec-v1\x00")
	input := bufio.NewReader(io.TeeReader(file, hash))
	var source io.Reader = input
	if magic, _ := input.Peek(2); string(magic) == "\x1f\x8b" {
		z, err := gzip.NewReader(input)
		if err != nil {
			return nil, err
		}
		defer z.Close()
		source = z
	}
	scan := bufio.NewScanner(source)
	scan.Buffer(make([]byte, 4096), 1<<20)
	if !scan.Scan() {
		return nil, fmt.Errorf("missing embedding header")
	}
	header := strings.Fields(scan.Text())
	if len(header) != 2 {
		return nil, fmt.Errorf("invalid embedding header")
	}
	size, sizeErr := strconv.Atoi(header[0])
	dim, dimErr := strconv.Atoi(header[1])
	if sizeErr != nil || dimErr != nil || size < 1 || dim < 1 || dim > 4096 {
		return nil, fmt.Errorf("invalid embedding dimensions")
	}
	model := &embeddings{dim: dim, words: make(map[string][]float32)}
	for scan.Scan() {
		fields := strings.Fields(scan.Text())
		if len(fields) != dim+1 {
			return nil, fmt.Errorf("invalid vector on row %d", len(model.words)+2)
		}
		if _, exists := model.words[fields[0]]; exists {
			return nil, fmt.Errorf("duplicate embedding word %q", fields[0])
		}
		vector := make([]float32, dim)
		for i, field := range fields[1:] {
			value, err := strconv.ParseFloat(field, 32)
			if err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
				return nil, fmt.Errorf("invalid embedding coordinate")
			}
			vector[i] = float32(value)
		}
		model.words[strings.Clone(fields[0])] = vector
	}
	if err := scan.Err(); err != nil {
		return nil, err
	}
	if len(model.words) != size {
		return nil, fmt.Errorf("expected %d word vectors, read %d", size, len(model.words))
	}
	return &encoder{model.encode, dim, [sha256.Size]byte(hash.Sum(nil)), 0.45}, nil
}

func (m *embeddings) encode(text string) []float32 {
	sum := make([]float64, m.dim)
	for _, term := range words(text) {
		lower := strings.ToLower(term)
		switch lower {
		case "a", "an", "and", "are", "as", "at", "be", "by", "for", "from", "in", "is", "it", "of", "on", "or", "that", "the", "to", "was", "with":
			continue
		}
		vector := m.words[term]
		if vector == nil && term != lower {
			vector = m.words[lower]
		}
		for i, v := range vector {
			sum[i] += float64(v)
		}
	}
	var norm float64
	for _, v := range sum {
		norm += v * v
	}
	if norm == 0 {
		return nil
	}
	norm = math.Sqrt(norm)
	vector := make([]float32, m.dim)
	for i, v := range sum {
		vector[i] = float32(v / norm)
	}
	return vector
}

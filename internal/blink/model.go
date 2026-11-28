// Package blink encodes text with the local Model2Vec Blink int8 bundle.
package blink

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
)

const Dimension = 256
const encodingVersion = "bigbrain-blink-int8-v1"

type Model struct {
	tokenizer   tokenizer
	table       []byte // signed int8 coordinates, row-major
	Fingerprint [sha256.Size]byte
}

func readFile(path string, limit int64) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > limit {
		return nil, fmt.Errorf("%s: expected a regular model file of 1–%d bytes", path, limit)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data := make([]byte, info.Size())
	if _, err := io.ReadFull(f, data); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return data, nil
}

// Load requires a single int8 table with a row for every token ID.
// Hash the exact bytes used for inference, including the tokenizer and config.
func Load(dir string) (*Model, error) {
	cfg, err := readFile(filepath.Join(dir, "config.json"), 64<<10)
	if err != nil {
		return nil, err
	}
	var config struct {
		Normalize bool            `json:"normalize"`
		DType     string          `json:"embedding_dtype"`
		MaxLength json.RawMessage `json:"max_length"`
	}
	if err := json.Unmarshal(cfg, &config); err != nil {
		return nil, fmt.Errorf("config.json: %w", err)
	}
	if !config.Normalize || config.DType != "int8" {
		return nil, fmt.Errorf("config.json: expected normalized int8 embeddings")
	}
	// Model2Vec defaults to 512 when the key is absent. Explicit null
	// selects unlimited length, as in the published Blink configuration.
	limit := 512
	if string(config.MaxLength) == "null" {
		limit = 0
	} else if len(config.MaxLength) > 0 && (json.Unmarshal(config.MaxLength, &limit) != nil || limit <= 0 || limit > 1<<30) {
		return nil, fmt.Errorf("config.json: invalid max_length")
	}
	rules, err := readFile(filepath.Join(dir, "tokenizer.json"), 32<<20)
	if err != nil {
		return nil, err
	}
	tokenizer, err := parseTokenizer(rules, limit)
	if err != nil {
		return nil, fmt.Errorf("tokenizer.json: %w", err)
	}
	weights, err := readFile(filepath.Join(dir, "model.safetensors"), 128<<20)
	if err != nil {
		return nil, err
	}
	table, err := int8Table(weights, tokenizer.rows)
	if err != nil {
		return nil, fmt.Errorf("model.safetensors: %w", err)
	}
	hash := sha256.New()
	io.WriteString(hash, encodingVersion)
	for _, data := range [][]byte{cfg, rules, weights} {
		digest := sha256.Sum256(data)
		hash.Write(digest[:])
	}
	return &Model{tokenizer: tokenizer, table: table, Fingerprint: [sha256.Size]byte(hash.Sum(nil))}, nil
}

func int8Table(data []byte, rows int) ([]byte, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("missing safetensors header")
	}
	n := binary.LittleEndian.Uint64(data)
	if n == 0 || n > 1<<20 || n > uint64(len(data)-8) {
		return nil, fmt.Errorf("invalid safetensors header length")
	}
	var header map[string]json.RawMessage
	if err := json.Unmarshal(data[8:8+n], &header); err != nil {
		return nil, err
	}
	delete(header, "__metadata__")
	var tensor struct {
		Dtype   string
		Shape   []int
		Offsets []int `json:"data_offsets"`
	}
	if len(header) != 1 || json.Unmarshal(header["embeddings"], &tensor) != nil || tensor.Dtype != "I8" ||
		len(tensor.Shape) != 2 || tensor.Shape[0] != rows || tensor.Shape[1] != Dimension {
		return nil, fmt.Errorf("expected one I8 embeddings tensor with shape [%d, %d]", rows, Dimension)
	}
	table := data[8+n:]
	if len(tensor.Offsets) != 2 || tensor.Offsets[0] != 0 || tensor.Offsets[1] != len(table) || len(table) != rows*Dimension {
		return nil, fmt.Errorf("embedding offsets or file size do not match the table shape")
	}
	return table, nil
}

// Encode returns the reference's zero vector and false when there is no
// usable direction. Keep the table packed and widen only the selected rows.
func (m *Model) Encode(text string) ([Dimension]float32, bool) {
	ids := m.tokenizer.ids(text)
	var sum [Dimension]int64
	for _, id := range ids {
		row := m.table[id*Dimension : (id+1)*Dimension]
		for j, v := range row {
			sum[j] += int64(int8(v))
		}
	}
	var vector [Dimension]float32
	var squared float64
	for _, v := range sum {
		squared += float64(v) * float64(v)
	}
	if squared == 0 {
		return vector, false
	}
	// Dividing the sum by the token count cancels in L2 normalization.
	norm := math.Sqrt(squared)
	for j, v := range sum {
		vector[j] = float32(float64(v) / norm)
	}
	return vector, true
}

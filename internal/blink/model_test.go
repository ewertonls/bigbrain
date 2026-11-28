package blink

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func syntheticBundle(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	rules, err := os.ReadFile("testdata/tokenizer.json")
	if err != nil {
		t.Fatal(err)
	}
	tok, err := parseTokenizer(rules, 0)
	if err != nil {
		t.Fatal(err)
	}
	rows := tok.rows
	header := fmt.Sprintf(`{"embeddings":{"dtype":"I8","shape":[%d,256],"data_offsets":[0,%d]}}`, rows, rows*Dimension)
	header += strings.Repeat(" ", (8-len(header)%8)%8)
	weights := binary.LittleEndian.AppendUint64(nil, uint64(len(header)))
	weights = append(weights, header...)
	for i := range rows {
		for j := range Dimension {
			weights = append(weights, byte((i*17+j*7)%251-125))
		}
	}
	for name, data := range map[string][]byte{"config.json": []byte(`{"normalize":true,"embedding_dtype":"int8","max_length":null}`), "tokenizer.json": rules, "model.safetensors": weights} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestModel2VecGoldens(t *testing.T) {
	checkGoldens(t, syntheticBundle(t), "testdata/synthetic.json")
}

func TestReleaseGoldens(t *testing.T) {
	dir := os.Getenv("BIGBRAIN_BLINK_BUNDLE")
	if dir == "" {
		t.Skip("set BIGBRAIN_BLINK_BUNDLE to the verified Blink release")
	}
	checkGoldens(t, dir, "testdata/blink.json")
}

func checkGoldens(t *testing.T, dir, path string) {
	t.Helper()
	model, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Cases []struct {
			Name, Text string
			MaxLength  *int `json:"max_length"`
			IDs        []int
			F32LE      []byte
		}
	}
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	if len(golden.Cases) == 0 {
		t.Fatal("empty golden file")
	}
	for _, c := range golden.Cases {
		t.Run(c.Name, func(t *testing.T) {
			m := *model
			if c.MaxLength != nil {
				data, err := os.ReadFile(filepath.Join(dir, "tokenizer.json"))
				if err != nil {
					t.Fatal(err)
				}
				m.tokenizer, err = parseTokenizer(data, *c.MaxLength)
				if err != nil {
					t.Fatal(err)
				}
			}
			if got := m.tokenizer.ids(c.Text); !slices.Equal(got, c.IDs) {
				t.Fatalf("token IDs %v, want %v", got, c.IDs)
			}
			got, usable := m.Encode(c.Text)
			if len(c.F32LE) != 4*len(got) {
				t.Fatalf("reference has %d bytes", len(c.F32LE))
			}
			any := false
			for i, v := range got {
				want := math.Float32frombits(binary.LittleEndian.Uint32(c.F32LE[i*4:]))
				if math.IsNaN(float64(v)) || math.Abs(float64(v-want)) > 1e-6 {
					t.Fatalf("coordinate %d = %g, reference %g", i, v, want)
				}
				any = any || want != 0
			}
			if any != usable {
				t.Fatalf("vector availability = %v, want %v", usable, any)
			}
		})
	}
}

func TestBundleIdentityAndMalformedFiles(t *testing.T) {
	dir := syntheticBundle(t)
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"config.json", "tokenizer.json", "model.safetensors"} {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if name == "model.safetensors" {
			data[len(data)-1] ^= 1
		} else {
			data = append(data, '\n')
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		changed, err := Load(dir)
		if err != nil || changed.Fingerprint == m.Fingerprint {
			t.Fatalf("%s fingerprint failed: %v", name, err)
		}
		m = changed
		if err := os.WriteFile(path, data[:len(data)/2], 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), name) {
			t.Fatalf("%s malformed diagnostic: %v", name, err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

// Outputs from tokenizers 0.23.3 BertNormalizer with strip_accents=true,
// lowercase=false. The long run crosses Go NFD's stream-safe boundary.
func TestBertAccentNormalization(t *testing.T) {
	cases := map[string]string{
		"á\u0327": "a", "İ": "I", "각": "\u1100\u1161\u11a8",
		"\U000105c9": "\U000105c9",
		"α\U0001d16d" + strings.Repeat("\u0301", 40) + "\u302e": "α\u302e\U0001d16d",
	}
	for text, want := range cases {
		if got := stripAccents(text); got != want {
			t.Errorf("%q → %q, want %q", text, got, want)
		}
	}
}

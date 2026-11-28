package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

// Sweep only the historical fixture. The separately authored held-out set is
// evaluated through the compiled CLI after choosing the threshold.
func TestBlinkCutoffCalibration(t *testing.T) {
	bundle := os.Getenv("BIGBRAIN_BLINK_BUNDLE")
	if bundle == "" {
		t.Skip("set BIGBRAIN_BLINK_BUNDLE to the verified release")
	}
	t.Setenv("BIGBRAIN_EMBEDDINGS", bundle)
	t.Setenv("BIGBRAIN_VECTOR_INDEX", "")
	data, err := os.ReadFile("../../tests/memory/fixture.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Project   string
		Limit     int
		Memories  []struct{ Key, Scope, Text, Source string }
		Links     [][2]string
		Questions []struct {
			Search, Expected string
			Relevant         []string
		}
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	s := openTestStore(t, filepath.Join(t.TempDir(), "calibration.lbug"))
	defer s.Close()
	model, fallback := s.vectorModel()
	if model == nil {
		t.Fatal(fallback)
	}
	ids, keys, sources, scopes := map[string]string{}, map[string]string{}, map[string]string{}, map[string]string{}
	for _, m := range fixture.Memories {
		scope := m.Scope
		if scope != "global" {
			scope = "project:" + scope
		}
		id := recordForTest(t, s, protocol.RecordRequest{Scope: scope, Key: m.Key, Text: m.Text, Source: m.Source}).ID
		ids[m.Key], keys[id], sources[m.Key], scopes[m.Key] = id, m.Key, m.Source, scope
	}
	for _, edge := range fixture.Links {
		scope := scopes[edge[0]]
		if scope == "global" {
			scope = scopes[edge[1]]
		}
		linkForTest(t, s, protocol.LinkRequest{Key: edge[0] + "-" + edge[1], Scope: scope, FromID: ids[edge[0]], ToID: ids[edge[1]], Kind: "related", Weight: 1, Source: "Fixture review"})
	}
	bestHits, fewestIrrelevant := -1, 0
	var winners []float64
	for step := 1; step <= 40; step++ {
		cut := float64(step) / 40
		model.minCosine = cut
		hits, irrelevant, returned := 0, 0, 0
		for _, q := range fixture.Questions {
			result := queryForTest(t, s, protocol.QueryRequest{Scope: "project:" + fixture.Project, Mode: "hybrid", Text: q.Search, IncludeGlobal: true, Limit: fixture.Limit})
			if result.Fallback != "" {
				t.Fatal(result.Fallback)
			}
			for _, m := range result.Matches {
				returned++
				key := keys[m.ID]
				if key == q.Expected && m.Source == sources[q.Expected] {
					hits++
				}
				if !slices.Contains(q.Relevant, key) {
					irrelevant++
				}
			}
		}
		t.Logf("cutoff %.3f: %d/%d expected sources, %d/%d irrelevant", cut, hits, len(fixture.Questions), irrelevant, returned)
		if hits > bestHits || hits == bestHits && irrelevant < fewestIrrelevant {
			bestHits, fewestIrrelevant = hits, irrelevant
			winners = nil
		}
		if hits == bestHits && irrelevant == fewestIrrelevant {
			winners = append(winners, cut)
		}
	}
	t.Logf("best cutoffs %v: %d/%d expected sources, %d irrelevant", winners, bestHits, len(fixture.Questions), fewestIrrelevant)
	if !slices.Contains(winners, blinkMinCosine) {
		t.Errorf("configured cutoff %.3f is outside the best-performing values", blinkMinCosine)
	}
}

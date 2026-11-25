package protocol

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"testing"
)

func TestLinkConfidenceRange(t *testing.T) {
	r := LinkRequest{Scope: "global", Key: "link", FromID: "a", ToID: "b", Kind: "supports", Source: "review", Weight: 7}
	for _, value := range []float64{0, 0.25, 1} {
		r.Confidence = &value
		if err := r.Validate(); err != nil {
			t.Errorf("confidence %v: %v", value, err)
		}
	}
	for _, value := range []float64{-0.1, 1.1, math.Inf(-1), math.Inf(1), math.NaN()} {
		r.Confidence = &value
		if err := r.Validate(); err == nil {
			t.Errorf("accepted confidence %v", value)
		}
	}
}

func TestQueryTraversalLimits(t *testing.T) {
	tests := []struct {
		name  string
		query QueryRequest
		valid bool
	}{
		{"default mode", QueryRequest{Text: "orbit", MaxDepth: new(1), MaxNeighbors: new(1)}, true},
		{"maximums", QueryRequest{Text: "orbit", Mode: "associative", MaxDepth: new(3), MaxNeighbors: new(10)}, true},
		{"text only", QueryRequest{Text: "orbit", Mode: "text"}, true},
		{"lookup", QueryRequest{ID: "a"}, true},
		{"zero depth", QueryRequest{Text: "orbit", MaxDepth: new(0)}, false},
		{"excess depth", QueryRequest{Text: "orbit", MaxDepth: new(4)}, false},
		{"zero neighbors", QueryRequest{Text: "orbit", MaxNeighbors: new(0)}, false},
		{"excess neighbors", QueryRequest{Text: "orbit", MaxNeighbors: new(11)}, false},
		{"depth on text only", QueryRequest{Text: "orbit", Mode: "text", MaxDepth: new(1)}, false},
		{"neighbors on lookup", QueryRequest{ID: "a", Mode: "hybrid", MaxNeighbors: new(1)}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.query.Scope = "global"
			if err := test.query.Validate(); (err == nil) != test.valid {
				t.Errorf("Validate() = %v, want valid %v", err, test.valid)
			}
		})
	}
}

func TestResponseLimitCoversEscapedMultiHopMatches(t *testing.T) {
	// The default JSON encoder expands '&' to six bytes.
	source := strings.Repeat("&", maxSourceBytes)
	scope := "project:" + strings.Repeat("a", 128)
	const timestamp = "2026-11-25T13:04:05.123456789Z"
	steps := make([]Path, MaxGraphDepth)
	for i := range steps {
		steps[i] = Path{
			RelationshipID: "R" + strconv.Itoa(i), FromID: "M" + strconv.Itoa(i), ToID: "M" + strconv.Itoa(i+1),
			Kind: strings.Repeat("&", 256), Scope: scope, Direction: "forward", Source: source,
			Weight: math.MaxFloat64, Confidence: new(1.0), ActivationCount: new(int64(1)),
			LastActivatedAt: timestamp, CreatedAt: timestamp,
		}
	}
	last := &steps[len(steps)-1]
	match := Match{
		Memory: Memory{
			ID: last.ToID, Scope: scope, Text: strings.Repeat("&", MaxMemoryTextBytes),
			Source: source, Type: "observation", RecordedAt: timestamp,
		},
		Reason: Reason{Kind: "relationship", GraphScore: 0.5, Steps: steps, Path: last, Hops: len(steps)},
	}
	encoded, err := json.Marshal(match)
	if err != nil {
		t.Fatal(err)
	}
	// Calculate the exact wire size without allocating a full-size page.
	var envelope bytes.Buffer
	if err := Encode(&envelope, QueryResult{Count: MaxQueryLimit, Matches: []Match{}}, nil); err != nil {
		t.Fatal(err)
	}
	size := envelope.Len() + MaxQueryLimit*len(encoded) + MaxQueryLimit - 1
	if size > MaxResponseBytes {
		t.Fatalf("full query response needs %d bytes, client limit is %d", size, MaxResponseBytes)
	}
}

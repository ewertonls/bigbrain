package main

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

func TestGraphRendersReturnedPathsAndEscapesProvenance(t *testing.T) {
	first := protocol.Path{RelationshipID: "r1", FromID: "b", ToID: "a", Kind: "cites", Direction: "reverse"}
	last := protocol.Path{RelationshipID: "r2", FromID: "b", ToID: "c", Kind: "supports", Direction: "forward",
		Source: `"><img src=x>`, ActivationCount: new(int64)}
	result := protocol.QueryResult{Count: 5, Matches: []protocol.Match{
		{Memory: protocol.Memory{ID: "b"}, Reason: protocol.Reason{Kind: "relationship", Path: &first}},
		{Memory: protocol.Memory{ID: "c", Text: "<script>stop()</script>"}, Reason: protocol.Reason{
			Kind: "relationship", Steps: []protocol.Path{first, last}, Path: &last, Hops: 2}},
	}}
	var page strings.Builder
	if err := writeGraph(&page, protocol.QueryRequest{Scope: "project:alpha", Text: "test"}, result); err != nil {
		t.Fatal(err)
	}
	html := page.String()
	for _, want := range []string{
		`Mode <code>associative</code>`,
		`data-count="5" data-shown="2" data-node-count="3" data-edge-count="2"`,
		`data-direction="reverse" data-from-id="b" data-to-id="a" data-travel-from="a" data-travel-to="b"`,
		`data-direction="forward" data-from-id="b" data-to-id="c" data-travel-from="b" data-travel-to="c"`,
		`data-step="2" data-relationship-id="r2"`, `href="#memory-c"`, `<dt>Activations</dt><dd>0</dd>`,
		"&lt;script&gt;stop()&lt;/script&gt;", "&lt;img src=x&gt;",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, unwanted := range []string{`href="#memory-a"`, "<script>", "<img "} {
		if strings.Contains(html, unwanted) {
			t.Errorf("unexpected %q", unwanted)
		}
	}
	if n := strings.Count(html, `data-relationship-id="r2"`); n != 1 {
		t.Errorf("rendered terminal step %d times, want 1", n)
	}
}

func TestGraphReportsOutputFailure(t *testing.T) {
	reader, writer := io.Pipe()
	reader.Close()
	defer writer.Close()
	err := writeGraph(writer, protocol.QueryRequest{}, protocol.QueryResult{})
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("writeGraph error = %v, want %v", err, io.ErrClosedPipe)
	}
}

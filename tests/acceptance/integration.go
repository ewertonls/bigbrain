package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/ewertonls/bigbrain/internal/protocol"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func verifyIntegration(bin, root string) []string {
	bin, err := filepath.Abs(bin)
	check(err)
	verifyAgentHandoff(cli{bin: bin, db: filepath.Join(root, "handoff")}, filepath.Join(filepath.Dir(bin), "..", "integrations"))
	verifyAdapterAndGraph(cli{bin: bin, db: filepath.Join(root, "adapter")})
	return []string{"agent_handoff", "mcp_cli_parity", "scoped_graph_view"}
}

func verifyAdapterAndGraph(c cli) {
	defer shutdown(c)
	f := seed(c, true)
	private := write(c, recordArgs("b", "Orbit private decision in B.", "B private source", "--project", "B")...).ID
	bridge := write(c, recordArgs("bridge", "The public quota statement.", "Public quota source", "--global")...).ID
	write(c, linkArgs("reverse", bridge, f.evidence)...)
	public := write(c, recordArgs("public", "Orbit public design.", "Public decision", "--global")...).ID
	trial := write(c, recordArgs("public-trial", "Design validation results.", "Public trial", "--global")...).ID
	write(c, "link", "--global", "--key", "global-link", "--from", public, "--to", trial, "--type", "verified_by", "--source", "Public review")
	write(c, "feedback", "--project", "A", "--key", "use-link", "--relationship", f.link, "--author", "Reviewer", "--source", "Prior usage")

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.bin, "--db", c.db, "mcp")
	cmd.Stderr = os.Stderr
	client := mcp.NewClient(&mcp.Implementation{Name: "acceptance", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	check(err)
	defer session.Close()
	call := func(tool string, args any) invocation {
		got, err := session.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
		check(err)
		body, err := json.Marshal(got.StructuredContent)
		check(err)
		var reply response
		check(json.Unmarshal(body, &reply))
		require(reply.Version == 1 && got.IsError == (reply.Error != nil) &&
			(reply.Error == nil) == (len(reply.Result) > 0), "invalid MCP tool result for %s: %s", tool, body)
		return invocation{envelope: reply}
	}
	request := protocol.RecordRequest{Scope: "project:A", Key: "mcp-record", Text: "A standalone observation.", Source: "Adapter trial"}
	first := decode[writeResult](call("record", request))
	retry := decode[writeResult](call("record", request))
	cliRetry := write(c, recordArgs(request.Key, request.Text, request.Source, "--project", "A")...)
	require(first.ID != "" && !first.Replayed && retry.ID == first.ID && retry.Replayed && retry == cliRetry, "inconsistent MCP/CLI write receipt")
	request.Text = "A changed request."
	expectError(call("record", request), "IDEMPOTENCY_CONFLICT")

	shutdown(c)
	before := snapshot(c)
	for _, test := range []struct {
		scope string
		flags []string
		count int
	}{
		{"project:A", []string{"--project", "A"}, 2},
		{"project:A", []string{"--project", "A", "--include-global"}, 5},
		{"global", []string{"--global"}, 2},
	} {
		args := append([]string{"--text", "Orbit", "--mode", "associative", "--max-depth", "2"}, test.flags...)
		want := decode[protocol.QueryResult](c.run(append([]string{"query"}, args...)...))
		require(want.Count == test.count, "unexpected scoped query for %v: %+v", test.flags, want)
		graphJSON := decode[protocol.QueryResult](c.run(append([]string{"graph"}, args...)...))
		require(reflect.DeepEqual(graphJSON, want), "graph --json differs from query for %v", test.flags)
		got := decode[protocol.QueryResult](call("query", protocol.QueryRequest{Scope: test.scope, Text: "Orbit",
			Mode: "associative", MaxDepth: new(2), IncludeGlobal: slices.Contains(test.flags, "--include-global")}))
		require(reflect.DeepEqual(got, want), "MCP response differs from CLI for %v", test.flags)
		html, err := exec.CommandContext(ctx, c.bin, append([]string{"--db", c.db, "graph"}, args...)...).Output()
		check(err)
		page := string(html)
		require(!strings.Contains(page, private) && !strings.Contains(page, "B private source"), "graph exposed project B")
		verifyGraphPage(page, want)
	}
	shutdown(c)
	require(reflect.DeepEqual(before, snapshot(c)), "MCP or graph reads altered the database")
}

// Compare each article with its match and each nested hop with that match's path.
func verifyGraphPage(page string, want protocol.QueryResult) {
	var document struct {
		Main struct {
			Count    int `xml:"data-count,attr"`
			Shown    int `xml:"data-shown,attr"`
			Nodes    int `xml:"data-node-count,attr"`
			Edges    int `xml:"data-edge-count,attr"`
			Articles []struct {
				ID         string   `xml:"h2>code"`
				Paragraphs []string `xml:"p"`
				Source     string   `xml:"dl>dd"`
				Steps      []struct {
					Number     int      `xml:"data-step,attr"`
					ID         string   `xml:"data-relationship-id,attr"`
					Direction  string   `xml:"data-direction,attr"`
					From       string   `xml:"data-from-id,attr"`
					To         string   `xml:"data-to-id,attr"`
					TravelFrom string   `xml:"data-travel-from,attr"`
					TravelTo   string   `xml:"data-travel-to,attr"`
					Fields     []string `xml:"details>dl>dd"`
				} `xml:"section>ol>li"`
			} `xml:"article"`
		} `xml:"body>main"`
	}
	dec := xml.NewDecoder(strings.NewReader(page))
	dec.Strict = false
	dec.AutoClose = xml.HTMLAutoClose
	dec.Entity = xml.HTMLEntity
	check(dec.Decode(&document))
	got := document.Main
	require(got.Count == want.Count && got.Shown == len(want.Matches) && len(got.Articles) == len(want.Matches), "graph shows incorrect result counts")
	nodes, edges := make(map[string]bool), make(map[string]bool)
	for i, match := range want.Matches {
		article := got.Articles[i]
		require(article.ID == match.ID && article.Source == match.Source && slices.Contains(article.Paragraphs, match.Text),
			"graph lost the text, ID or source of memory %s", match.ID)
		nodes[match.ID] = true
		path := match.Reason.Steps
		if len(path) == 0 && match.Reason.Path != nil {
			path = []protocol.Path{*match.Reason.Path}
		}
		require(len(article.Steps) == len(path), "wrong path length on memory %s", match.ID)
		for j, p := range path {
			step := article.Steps[j]
			from, to := p.FromID, p.ToID
			if p.Direction == "reverse" {
				from, to = to, from
			}
			require(step.Number == j+1 && step.ID == p.RelationshipID && step.Direction == p.Direction &&
				step.From == p.FromID && step.To == p.ToID && step.TravelFrom == from && step.TravelTo == to &&
				slices.Contains(step.Fields, p.Source), "graph lost provenance or direction on hop %d of memory %s", j+1, match.ID)
			nodes[p.FromID], nodes[p.ToID], edges[p.RelationshipID] = true, true, true
		}
	}
	require(got.Nodes == len(nodes) && got.Edges == len(edges), "graph shows incorrect node or relationship counts")
}

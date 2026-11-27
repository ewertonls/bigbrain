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
	"syscall"
	"time"

	"github.com/ewertonls/bigbrain/internal/protocol"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func verifyIntegration(bin, root string) []string {
	bin, err := filepath.Abs(bin)
	check(err)
	verifyAgentHandoff(cli{bin: bin, db: filepath.Join(root, "handoff")}, filepath.Join(filepath.Dir(bin), "..", "integrations"))
	verifyAdapterAndGraph(cli{bin: bin, db: filepath.Join(root, "adapter")})
	verifyDirectoryProject(cli{bin: bin, db: filepath.Join(root, "directory-db"), dir: filepath.Join(root, "directory-project")})
	verifyReviewedHandoff(cli{bin: bin, db: filepath.Join(root, "plugin-db"), dir: filepath.Join(root, "plugin-project")})
	verifyExplorerPanel(cli{bin: bin, db: filepath.Join(root, "panel-db"), dir: filepath.Join(root, "panel-project")})
	return []string{"explorer_conversation_panel", "reviewed_plugin_handoff", "agent_handoff", "mcp_cli_parity", "scoped_graph_view", "directory_project"}
}

func verifyDirectoryProject(c cli) {
	defer shutdown(c)
	c.env = []string{"GIT_TRACE=1"}
	child := filepath.Join(c.dir, "docs")
	check(os.MkdirAll(child, 0700))
	decode[json.RawMessage](c.run("project", "init", "orbit"))
	id, err := os.ReadFile(filepath.Join(c.dir, ".bigbrain-project"))
	check(err)
	require(string(id) == "orbit\n", "init stored unexpected project ID: %q", id)
	c.dir = child
	local := write(c, recordArgs("local", "Project decision", "Project notes")...).ID
	global := write(c, recordArgs("global", "Shared practice", "Guide", "--global")...).ID
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	session, call := connectMCP(ctx, c)
	defer session.Close()
	queryIDs(decode[matches](call("query", map[string]string{"id": local})), local)
	queryIDs(decode[matches](call("query", map[string]string{"id": global})), global)
	saved := decode[writeResult](call("record", map[string]string{"key": "mcp", "text": "Another decision", "source": "Notes"}))
	queryIDs(query(c, "--project", "orbit", "--id", saved.ID), saved.ID)
	queryIDs(query(c, "--global", "--id", saved.ID))

	// Malformed local markers must produce a diagnostic instead of hanging or
	// silently selecting the parent project.
	marker := filepath.Join(child, ".bigbrain-project")
	check(syscall.Mkfifo(marker, 0600))
	expectError(c.run("db-path"), "PROJECT_CONFIG_ERROR")
	check(os.Remove(marker))
	check(os.Symlink("missing-project-file", marker))
	expectError(c.run("project", "init", "nested"), "PROJECT_CONFIG_ERROR")
	check(os.Remove(marker))

	// A nested Git repository uses its own worktree configuration.
	git(child, "init", "-q")
	expectError(c.run("db-path"), "PROJECT_NOT_CONFIGURED")
	c.env = []string{"PATH=" + filepath.Join(child, "no-bin")}
	expectError(c.run("db-path"), "PROJECT_UNAVAILABLE")
	expectError(c.run("project", "init", "nested"), "PROJECT_UNAVAILABLE")
	queryIDs(query(c, "--project", "orbit", "--id", saved.ID), saved.ID)
}

func verifyAdapterAndGraph(c cli) {
	defer shutdown(c)
	c.dir = c.db + "-repo"
	check(os.Mkdir(c.dir, 0700))
	git(c.dir, "init", "-q")
	decode[json.RawMessage](c.run("project", "init", "A"))
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
	session, call := connectMCP(ctx, c)
	defer session.Close()
	request := protocol.RecordRequest{Scope: "project:A", Key: "mcp-record", Text: "A standalone observation.", Source: "Adapter trial"}
	first := decode[writeResult](call("record", map[string]string{"key": request.Key, "text": request.Text, "source": request.Source}))
	retry := decode[writeResult](call("record", request))
	cliRetry := write(c, recordArgs(request.Key, request.Text, request.Source, "--project", "A")...)
	require(first.ID != "" && !first.Replayed && retry.ID == first.ID && retry.Replayed && retry == cliRetry, "inconsistent MCP/CLI write receipt")
	request.Text = "A changed request."
	expectError(call("record", request), "IDEMPOTENCY_CONFLICT")
	queryIDs(decode[matches](call("query", map[string]string{"id": first.ID})), first.ID)
	queryIDs(decode[matches](call("query", map[string]string{"id": public})), public)
	queryIDs(decode[matches](call("query", map[string]string{"id": private})))
	queryIDs(decode[matches](call("query", map[string]any{"id": public, "include_global": false})))
	expectError(call("query", map[string]string{"scope": "", "id": public}), "INVALID_SCOPE")
	expectError(call("review-weight", map[string]any{}), "INVALID_SCOPE")

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

	// A broken repository default must leave explicit scopes usable on restart.
	check(os.WriteFile(filepath.Join(c.dir, ".git", "bigbrain-project"), []byte("invalid id\n"), 0600))
	restarted, callExplicit := connectMCP(ctx, c)
	defer restarted.Close()
	expectError(callExplicit("record", map[string]string{"key": "missing-scope", "text": "Draft", "source": "Test"}), "INVALID_SCOPE")
	queryIDs(decode[matches](callExplicit("query", map[string]string{"scope": "project:A", "id": first.ID})), first.ID)
}

func startMCPClient(ctx context.Context, c cli, args ...string) *mcp.ClientSession {
	cmd := exec.CommandContext(ctx, c.bin, append([]string{"--db", c.db, "mcp"}, args...)...)
	cmd.Dir = c.dir
	cmd.Stderr = os.Stderr
	client := mcp.NewClient(&mcp.Implementation{Name: "acceptance", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	check(err)
	return session
}

func connectMCP(ctx context.Context, c cli) (*mcp.ClientSession, func(string, any) invocation) {
	session := startMCPClient(ctx, c)
	return session, func(tool string, args any) invocation {
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

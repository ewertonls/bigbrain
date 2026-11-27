package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ewertonls/bigbrain/internal/protocol"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func verifyExplorerPanel(c cli) {
	defer shutdown(c)
	check(os.Mkdir(c.dir, 0700))
	decode[json.RawMessage](c.run("project", "init", "A"))
	id := write(c, recordArgs("transport", "Use a local socket.", "docs/transport.md")...).ID
	c.env = []string{"BIGBRAIN_CODEX_STATE=" + c.db + "-state", "CODEX_THREAD_ID=panel-session"}
	decode[json.RawMessage](c.run("context", "--id", id))
	draft := decode[struct {
		ID       string
		Revision int
	}](c.invokeInput(strings.NewReader(`{"producer":"panel fixture","memory":{"type":"decision","text":"Approve this version.","source":"fixture/review"}}`), "--json", "handoff", "stage"))
	workspace := c.dir
	c.dir = filepath.Dir(workspace) // MCP can start in a plugin cache outside the project.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	session := startMCPClient(ctx, c, "--explorer-workspace", workspace)
	defer session.Close()
	list, err := session.ListTools(ctx, nil)
	check(err)
	require(len(list.Tools) == 6, "panel registered unexpected tools: %v", list.Tools)
	var uri string
	for _, tool := range list.Tools {
		require(tool.Annotations != nil && tool.Annotations.ReadOnlyHint, "unexpected writable panel tool %s", tool.Name)
		if tool.Name == "explorer" {
			metadata, err := json.Marshal(tool.Meta)
			check(err)
			var advertised struct {
				UI struct {
					ResourceURI string `json:"resourceUri"`
				} `json:"ui"`
				OpenAI struct{ Entrypoints []struct{ Type string } } `json:"openai/ui"`
			}
			check(json.Unmarshal(metadata, &advertised))
			require(len(advertised.OpenAI.Entrypoints) == 1 && advertised.OpenAI.Entrypoints[0].Type == "thread", "missing conversation entrypoint")
			uri = advertised.UI.ResourceURI
		}
	}
	resource, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
	check(err)
	require(len(resource.Contents) == 1 && resource.Contents[0].MIMEType == "text/html;profile=mcp-app" && strings.Contains(resource.Contents[0].Text, "Knowledge network"), "missing bundled Explorer")
	for _, previous := range []string{"ui://bigbrain/explorer-v2.html", "ui://bigbrain/card-v1.html"} {
		resource, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: previous})
		check(err)
		require(len(resource.Contents) == 1 && resource.Contents[0].URI == previous, "old conversation resource unavailable")
	}
	call := func(name string, input any) *mcp.CallToolResult {
		r, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: input})
		check(err)
		return r
	}
	require(!call("explorer", map[string]any{}).IsError, "desktop entrypoint rejected empty input")
	q := protocol.QueryRequest{Scope: "project:A", ID: id, Limit: 1}
	got := call("explorer_query", q)
	payload, err := json.Marshal(got.Meta["bigbrain"])
	check(err)
	var data protocol.QueryResult
	check(json.Unmarshal(payload, &data))
	require(!got.IsError && len(data.Matches) == 1 && data.Matches[0].Source == "docs/transport.md", "missing source in panel")
	visible, err := json.Marshal(got.Content)
	check(err)
	require(got.StructuredContent == nil && !strings.Contains(string(visible), "docs/transport.md"), "panel leaked the source to model content")
	card := call("recall_card", map[string]any{"text": "socket"})
	raw, err := json.Marshal(card.Meta["bigbrain"])
	check(err)
	require(!card.IsError && strings.Contains(string(raw), "docs/transport.md") && strings.Contains(string(raw), `"preview":true`), "missing contextual recall preview: %s", raw)
	require(call("explorer_browse", map[string]any{"scope": "project:B"}).IsError, "browse allowed another project")
	for _, writable := range []bool{false, true} {
		flags := []string{"--explorer-workspace", workspace, "--explorer-session", "panel-session"}
		if writable {
			flags = append(flags, "--explorer-review")
		}
		pinned := startMCPClient(ctx, c, flags...)
		pcall := func(tool string, input any) *mcp.CallToolResult {
			r, err := pinned.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: input})
			check(err)
			return r
		}
		card := pcall("recall_card", map[string]any{})
		raw, err := json.Marshal(card.Meta["bigbrain"])
		check(err)
		require(!card.IsError && strings.Contains(string(raw), "docs/transport.md") && strings.Contains(string(raw), `"preview":false`), "missing session receipt")
		card = pcall("review_card", map[string]any{})
		raw, err = json.Marshal(card.Meta["bigbrain"])
		check(err)
		require(!card.IsError && strings.Contains(string(raw), draft.ID), "missing staged card")
		list, err := pinned.ListTools(ctx, nil)
		check(err)
		hasSave := false
		for _, tool := range list.Tools {
			if tool.Name == "explorer_save" {
				hasSave = true
				meta, _ := json.Marshal(tool.Meta)
				require(!tool.Annotations.ReadOnlyHint && strings.Contains(string(meta), `"visibility":["app"]`), "review writes exposed to model")
			}
		}
		require(hasSave == writable, "review capability mismatch")
		if writable {
			require(pcall("explorer_save", map[string]any{"id": draft.ID, "revision": draft.Revision + 1}).IsError, "accepted an unreviewed revision")
			for range 2 {
				require(!pcall("explorer_save", map[string]any{"id": draft.ID, "revision": draft.Revision}).IsError, "save or replay failed")
			}
			require(queryText(cli{bin: c.bin, db: c.db, dir: workspace}, "Approve", "text", "5").Count == 1, "replay duplicated a reviewed memory")
		}
		pinned.Close()
	}

	q.Scope = "project:B"
	require(call("explorer_query", q).IsError, "panel accepted another project")
	check(os.WriteFile(filepath.Join(workspace, ".bigbrain-project"), []byte("B\n"), 0600))
	require(call("explorer_query", q).IsError, "panel followed a changed project binding")
}

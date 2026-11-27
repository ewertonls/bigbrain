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
	workspace := c.dir
	c.dir = filepath.Dir(workspace) // MCP can start in a plugin cache outside the project.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	session := startMCPClient(ctx, c, "--explorer-workspace", workspace)
	defer session.Close()
	list, err := session.ListTools(ctx, nil)
	check(err)
	require(len(list.Tools) == 3, "panel registered unexpected tools: %v", list.Tools)
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
	require(len(resource.Contents) == 1 && resource.Contents[0].MIMEType == "text/html;profile=mcp-app" && strings.Contains(resource.Contents[0].Text, "Search memories"), "missing bundled Explorer")
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
	q.Scope = "project:B"
	require(call("explorer_query", q).IsError, "panel accepted another project")
	check(os.WriteFile(filepath.Join(workspace, ".bigbrain-project"), []byte("B\n"), 0600))
	require(call("explorer_query", q).IsError, "panel followed a changed project binding")
}

package main

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const explorerURI = "ui://bigbrain/explorer.html"
const explorerMIME = "text/html;profile=mcp-app"

func serveExplorer(db, workspace string, in io.ReadCloser, out io.Writer) error {
	p := pluginOptions{cwd: workspace, include: true}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	b, err := p.bind(ctx, db)
	cancel()
	if err != nil {
		return err
	}
	v := viewer{binding: b, workspace: p, db: db}
	var html strings.Builder
	// Only bundled assets enter the template. Memory data travels through tools/call.
	err = viewerPages.ExecuteTemplate(&html, "panel.tmpl", nil)
	if err != nil {
		return err
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "bigbrain-explorer", Version: "1.0.0"}, nil)
	server.AddResource(&mcp.Resource{URI: explorerURI, Name: "BigBrain Explorer", MIMEType: explorerMIME}, func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: explorerURI, MIMEType: explorerMIME, Text: html.String()}}}, nil
	})
	open := &mcp.Tool{
		Name: "explorer", Title: "BigBrain Explorer",
		Description: "Open project memory search and source inspection beside this conversation.",
		Meta: mcp.Meta{
			"ui":        map[string]any{"resourceUri": explorerURI},
			"openai/ui": map[string]any{"entrypoints": []map[string]string{{"type": "thread"}}},
		},
	}
	addExplorerTool(server, &v, open, func(ctx context.Context, _ struct{}) (any, error) {
		state, err := lifecycle("status", b.Database)
		if err != nil {
			return nil, err
		}
		return map[string]any{"binding": map[string]any{"scope": b.Scope, "include_global": b.IncludeGlobal}, "daemon": state}, nil
	})
	appOnly := mcp.Meta{"ui": map[string]any{"visibility": []string{"app"}}}
	addExplorerTool(server, &v, &mcp.Tool{Name: "explorer_query", Description: "Search or inspect a visible memory.", Meta: appOnly}, v.query)
	addExplorerTool(server, &v, &mcp.Tool{Name: "explorer_history", Description: "Read visible relationships for a memory.", Meta: appOnly}, v.history)
	return server.Run(context.Background(), &mcp.IOTransport{Reader: in, Writer: mcpOutput{out}})
}

func addExplorerTool[T, R any](server *mcp.Server, v *viewer, tool *mcp.Tool, read func(context.Context, T) (R, error)) {
	tool.Annotations = &mcp.ToolAnnotations{ReadOnlyHint: true}
	mcp.AddTool(server, tool, func(ctx context.Context, _ *mcp.CallToolRequest, input T) (*mcp.CallToolResult, any, error) {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := v.checkBinding(ctx); err != nil {
			return nil, nil, err
		}
		data, err := read(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "BigBrain Explorer ready."}}, Meta: mcp.Meta{"bigbrain": data}}, nil, nil
	})
}

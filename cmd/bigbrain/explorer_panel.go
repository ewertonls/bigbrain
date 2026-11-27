package main

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/ewertonls/bigbrain/internal/protocol"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const explorerURI = "ui://bigbrain/explorer-v3.html"
const cardURI = "ui://bigbrain/card-v2.html"
const explorerMIME = "text/html;profile=mcp-app"

func serveExplorer(db, workspace, session string, review bool, in io.ReadCloser, out io.Writer) error {
	p := pluginOptions{cwd: workspace, include: true, session: session, state: os.Getenv("BIGBRAIN_CODEX_STATE")}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	b, err := p.bind(ctx, db)
	cancel()
	// Codex also discovers tools outside a project. Let that discovery succeed
	// and bind on first use if the chat's workspace is initialized later.
	if err != nil && !(workspace == "." && (errors.Is(err, errProjectUnavailable) || errors.Is(err, errProjectNotConfigured))) {
		return err
	}
	v := viewer{binding: b, workspace: p, db: db}
	if session != "" {
		v.dir, err = p.stateDir()
		if err != nil {
			return err
		}
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "bigbrain-explorer", Version: "2.0.0"}, nil)
	// Existing conversation cards can still carry a previous resource URI.
	for name, uris := range map[string][]string{"panel": {explorerURI, "ui://bigbrain/explorer-v2.html"}, "card": {cardURI, "ui://bigbrain/card-v1.html"}} {
		var html strings.Builder
		if err := viewerPages.ExecuteTemplate(&html, name+".tmpl", nil); err != nil {
			return err
		}
		for _, uri := range uris {
			server.AddResource(&mcp.Resource{URI: uri, Name: "BigBrain " + name, MIMEType: explorerMIME}, func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
				return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: explorerMIME, Text: html.String()}}}, nil
			})
		}
	}
	open := &mcp.Tool{Name: "explorer", Title: "BigBrain Explorer", Description: "Browse the bound project in a conversation side panel.",
		Meta: mcp.Meta{
			"ui":        map[string]any{"resourceUri": explorerURI, "visibility": []string{"app"}},
			"openai/ui": map[string]any{"entrypoints": []map[string]string{{"type": "thread"}}},
		},
	}
	addExplorerTool(server, &v, open, func(ctx context.Context, _ struct{}) (any, error) {
		b := v.binding
		state, err := lifecycle("status", b.Database)
		if err != nil {
			return nil, err
		}
		return map[string]any{"binding": map[string]any{"scope": b.Scope, "include_global": b.IncludeGlobal, "session_id": session}, "daemon": state}, nil
	})
	appOnly := mcp.Meta{"ui": map[string]any{"visibility": []string{"app"}}}
	addExplorerTool(server, &v, &mcp.Tool{Name: "explorer_query", Description: "Search visible memories.", Meta: appOnly}, v.query)
	addExplorerTool(server, &v, &mcp.Tool{Name: "explorer_history", Description: "Read a stored neighborhood.", Meta: appOnly}, v.history)
	addExplorerTool(server, &v, &mcp.Tool{Name: "explorer_browse", Description: "Page through recent visible memories.", Meta: appOnly}, v.browse)
	addExplorerTool(server, &v, &mcp.Tool{Name: "explorer_feedback", Description: "Read feedback recorded for a visible relationship.", Meta: appOnly}, v.feedbackHistory)
	card := mcp.Meta{"ui": map[string]any{"resourceUri": cardURI}}
	addExplorerTool(server, &v, &mcp.Tool{Name: "recall_card", Title: "BigBrain recall", Description: "Show the pinned session's latest emitted context receipt, or preview context for explicit search text. A preview does not emit context.", Meta: card}, v.recallCard)
	if session != "" {
		readDrafts := func(ctx context.Context, _ struct{}) (any, error) { return v.reviewCard(review) }
		addExplorerTool(server, &v, &mcp.Tool{Name: "review_card", Title: "BigBrain handoff review", Description: "Inspect pending handoff drafts in the pinned project and session.", Meta: card}, readDrafts)
		addExplorerTool(server, &v, &mcp.Tool{Name: "explorer_drafts", Meta: appOnly}, readDrafts)
		addExplorerTool(server, &v, &mcp.Tool{Name: "explorer_automation", Meta: appOnly}, func(context.Context, struct{}) (any, error) { return v.sessionAutomation(v.binding) })
		if review {
			for _, op := range []string{"edit", "save", "dismiss"} {
				tool := &mcp.Tool{Name: "explorer_" + op, Meta: appOnly, Annotations: &mcp.ToolAnnotations{ReadOnlyHint: false}}
				addExplorerTool(server, &v, tool, func(ctx context.Context, r draftAction) (any, error) {
					return v.applyDraft(ctx, op, r)
				})
			}
		}
	}
	return server.Run(context.Background(), &mcp.IOTransport{Reader: in, Writer: mcpOutput{out}})
}

func addExplorerTool[T, R any](server *mcp.Server, v *viewer, tool *mcp.Tool, handle func(context.Context, T) (R, error)) {
	if tool.Annotations == nil {
		tool.Annotations = &mcp.ToolAnnotations{ReadOnlyHint: true}
	}
	mcp.AddTool(server, tool, func(ctx context.Context, _ *mcp.CallToolRequest, input T) (*mcp.CallToolResult, any, error) {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := v.checkBinding(ctx); err != nil {
			return nil, nil, err
		}
		data, err := handle(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "BigBrain view ready."}}, Meta: mcp.Meta{"bigbrain": data}}, nil, nil
	})
}

type recallCardInput struct {
	Text          string `json:"text,omitempty"`
	IncludeGlobal bool   `json:"include_global,omitempty"`
}
type recallItem struct {
	ID     string `json:"id"`
	Scope  string `json:"scope,omitempty"`
	Type   string `json:"type,omitempty"`
	Source string `json:"source,omitempty"`
	Text   string `json:"text,omitempty"`
	Reason string `json:"reason,omitempty"`
}

func (v *viewer) recallCard(ctx context.Context, input recallCardInput) (any, error) {
	b := v.binding
	preview := input.Text != ""
	var receipt *recallReceipt
	visible := map[string]protocol.Memory{}
	if preview {
		b.IncludeGlobal = input.IncludeGlobal
		matches, fallback, err := recallMatches(ctx, b, v.dir, recallInput{Text: input.Text}, true)
		if err != nil {
			return nil, err
		}
		packet := projectContext(matches, fallback, nil)
		receipt = &packet.Receipt
		for _, m := range matches {
			visible[m.ID] = m.Memory
		}
	} else if b.Session != "" {
		s, err := readState[recallState](b.sessionFile(v.dir, "recall"))
		if err != nil {
			return nil, err
		}
		receipt = s.Last
		if receipt != nil {
			for _, entry := range receipt.Memories {
				h, err := v.history(ctx, protocol.HistoryRequest{Scope: b.Scope, IncludeGlobal: true, ID: entry.ID, Limit: 1})
				if err != nil {
					var e *protocol.Error
					if errors.As(err, &e) && e.Code == "NOT_FOUND" {
						continue
					}
					return nil, err
				}
				visible[entry.ID] = h.Memory
			}
		}
	}
	var items []recallItem
	if receipt != nil {
		for _, entry := range receipt.Memories {
			row := recallItem{ID: entry.ID, Reason: entry.Reason}
			m := visible[entry.ID]
			if m.ID != "" && stateKey(m.ID, m.Scope, m.Type, m.Text, m.Source) == entry.Hash {
				row.Text = excerpt(m.Text, 1200)
				row.Type = m.Type
				row.Scope = m.Scope
				row.Source = m.Source
			}
			items = append(items, row)
		}
	}
	return map[string]any{"kind": "recall", "preview": preview, "scope": b.Scope, "session": b.Session, "receipt": receipt, "items": items}, nil
}
func (v *viewer) reviewCard(writable bool) (any, error) {
	ds, err := readState[drafts](v.binding.sessionFile(v.dir, "drafts"))
	if err != nil {
		return nil, err
	}
	pending := make(drafts, 0)
	for _, d := range ds {
		if d.Status != "saved" && d.Status != "dismissed" {
			d.Manifest = nil
			pending = append(pending, d)
		}
	}
	return map[string]any{"kind": "review", "scope": v.binding.Scope, "session": v.binding.Session, "drafts": pending, "writable": writable}, nil
}

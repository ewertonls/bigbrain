package mcp

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/ewertonls/bigbrain/internal/protocol"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestToolCallsPreserveCLIContract(t *testing.T) {
	calls := 0
	server := NewServer("", func(_ context.Context, name string, in any) (json.RawMessage, error) {
		calls++
		switch req := in.(type) {
		case protocol.QueryRequest:
			if name != "query" || req.Mode != "hybrid" || req.Scope != "project:A" {
				t.Errorf("unexpected query: %+v", req)
			}
			return json.RawMessage(`{"count":0,"matches":[]}`), nil
		case protocol.LinkRequest:
			if name != "link" || req.Weight != 1 || req.Key != "saved-key" {
				t.Errorf("unexpected link: %+v", req)
			}
			return nil, protocol.NewError("IDEMPOTENCY_CONFLICT", "Key already used.")
		default:
			t.Errorf("unexpected daemon call: %s", name)
			return nil, nil
		}
	})
	ctx := t.Context()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	for _, tc := range []struct {
		name, args string
		code       string
	}{
		{"query", `{"scope":"project:A","text":"orbit"}`, ""},
		{"link", `{"scope":"project:A","key":"saved-key","from_id":"d","to_id":"e","type":"because","source":"review"}`, "IDEMPOTENCY_CONFLICT"},
		{"query", `{"text":"orbit"}`, "INVALID_SCOPE"},
		{"record", `{"scope":"project:A","text":"orbit","source":"review"}`, "INVALID_REQUEST"},
		{"query", `{"scope":"project:A","text":"orbit","typo":true}`, "INVALID_REQUEST"},
		{"query", `{"scope":"project:A","Scope":"global","text":"orbit"}`, "INVALID_REQUEST"},
		{"link", `{"scope":"project:A","key":"saved-key","from_id":"d","to_id":"e","type":"because","source":"review","weight":null}`, "INVALID_REQUEST"},
	} {
		result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tc.name, Arguments: json.RawMessage(tc.args)})
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(result.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		var envelope protocol.Response
		if err := json.Unmarshal(data, &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Version != protocol.Version || result.IsError != (tc.code != "") {
			t.Fatalf("%s: unexpected result: %s", tc.name, data)
		}
		if tc.code != "" && (envelope.Error == nil || envelope.Error.Code != tc.code) {
			t.Errorf("%s: got %s, want %s", tc.name, data, tc.code)
		}
		if tc.code == "" && string(envelope.Result) != `{"count":0,"matches":[]}` {
			t.Errorf("%s: unexpected daemon result: %s", tc.name, data)
		}
		if len(result.Content) != 1 {
			t.Fatalf("%s: unexpected tool content: %v", tc.name, result.Content)
		}
		var textEnvelope any
		content, ok := result.Content[0].(*mcp.TextContent)
		if !ok || json.Unmarshal([]byte(content.Text), &textEnvelope) != nil || !reflect.DeepEqual(textEnvelope, result.StructuredContent) {
			t.Errorf("%s: text and structured results differ", tc.name)
		}
	}
	if calls != 2 {
		t.Errorf("daemon called %d times, want 2", calls)
	}
}

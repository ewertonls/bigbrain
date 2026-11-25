package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

func TestJSONOutputWithHelpAndInvalidFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		json     bool
		wantCode string
	}{
		{"help command", []string{"help", "--json"}, true, ""},
		{"help flag", []string{"status", "--help", "--json"}, true, ""},
		{"JSON after invalid value", []string{"query", "--limit", "many", "--json=T"}, true, "INVALID_REQUEST"},
		{"explicitly disabled", []string{"--json", "query", "--limit", "many", "--json=false"}, false, ""},
		{"flag value is text", []string{"record", "--unknown", "--text", "--json"}, false, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := run(test.args, &stdout, &stderr)
			if !test.json {
				if err == nil || stdout.Len() != 0 || stderr.Len() == 0 {
					t.Fatalf("expected a text error, got stdout %q, stderr %q, err %v", &stdout, &stderr, err)
				}
				return
			}
			var response protocol.Response
			if decodeErr := json.Unmarshal(stdout.Bytes(), &response); decodeErr != nil {
				t.Fatalf("expected one JSON response, got %q: %v", &stdout, decodeErr)
			}
			if response.Version != protocol.Version || stderr.Len() != 0 {
				t.Fatalf("unexpected version or stderr: %+v, %q", response, &stderr)
			}
			if test.wantCode == "" {
				if err != nil || response.Error != nil || len(response.Result) == 0 {
					t.Fatalf("expected help, got %+v, err %v", response, err)
				}
			} else if err == nil || response.Error == nil || response.Error.Code != test.wantCode || response.Result != nil {
				t.Fatalf("expected %s, got %+v, err %v", test.wantCode, response, err)
			}
		})
	}
}

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

func TestDBPathBeforeFirstWrite(t *testing.T) {
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	dir := filepath.Join(data, "bigbrain")
	file := filepath.Join(dir, "memory.lbug")
	var stdout, stderr bytes.Buffer
	if err := run([]string{"db-path", "--project", "orbit"}, &stdout, &stderr); err != nil || stdout.String() != file+"\n" {
		t.Fatalf("db-path returned %q, %v", &stdout, err)
	}
	stdout.Reset()
	if err := run([]string{"--json", "db-path", "--global"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	var reply struct {
		Version int              `json:"version"`
		Result  databaseLocation `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &reply); err != nil {
		t.Fatal(err)
	}
	if reply.Version != protocol.Version || reply.Result != (databaseLocation{"global", dir, file}) {
		t.Fatalf("unexpected location: %+v", reply)
	}
	for _, flags := range [][]string{nil, {"--project", "orbit", "--global"}} {
		_, err := execute(append([]string{"db-path"}, flags...), &options{})
		var scopeErr *protocol.Error
		if !errors.As(err, &scopeErr) || scopeErr.Code != "INVALID_SCOPE" {
			t.Fatalf("expected scope error for %v, got %v", flags, err)
		}
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("db-path should leave the database directory absent, got %v", err)
	}
}

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

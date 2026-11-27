package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

func TestDBPathBeforeFirstWrite(t *testing.T) {
	data := t.TempDir()
	t.Chdir(data)
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
	for _, tc := range []struct {
		flags []string
		code  string
	}{
		{nil, "PROJECT_UNAVAILABLE"},
		{[]string{"--project", "orbit", "--global"}, "INVALID_SCOPE"},
		{[]string{"--project="}, "INVALID_SCOPE"},
	} {
		_, err := execute(append([]string{"db-path"}, tc.flags...), &options{})
		var scopeErr *protocol.Error
		if !errors.As(err, &scopeErr) || scopeErr.Code != tc.code {
			t.Fatalf("expected %s for %v, got %v", tc.code, tc.flags, err)
		}
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("db-path should leave the database directory absent, got %v", err)
	}
}

func TestDirectoryProjectsWithoutGitExecutable(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PATH", t.TempDir())
	t.Chdir(root)
	for range 2 {
		if _, err := initProject("parent"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := initProject("other"); err == nil {
		t.Fatal("init replaced an existing project")
	}
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(child)
	if scope, err := configuredProjectScope(); err != nil || scope != "project:parent" {
		t.Fatalf("inherited scope = %q, err = %v", scope, err)
	}
	if _, err := initProject("child"); err != nil {
		t.Fatal(err)
	}
	if scope, err := configuredProjectScope(); err != nil || scope != "project:child" {
		t.Fatalf("nested scope = %q, err = %v", scope, err)
	}
	if err := os.WriteFile(localProjectFile, []byte("bad id\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := configuredProjectScope()
	var problem *protocol.Error
	if !errors.As(err, &problem) || problem.Code != "PROJECT_CONFIG_ERROR" {
		t.Fatalf("invalid nested project file: got %v", err)
	}
}

func TestCommandHelp(t *testing.T) {
	for _, args := range [][]string{
		{"query", "-h"}, {"query", "--help"}, {"help", "query"},
		{"--json", "query", "-h"}, {"help", "query", "--json"},
	} {
		var stdout, stderr bytes.Buffer
		if err := run(args, &stdout, &stderr); err != nil || stderr.Len() != 0 {
			t.Fatalf("%v: %v, stderr %q", args, err, &stderr)
		}
		help := stdout.String()
		if strings.HasPrefix(help, "{") {
			var response protocol.Response
			if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(response.Result, &help); err != nil {
				t.Fatal(err)
			}
		}
		if !strings.HasPrefix(help, "Usage: bigbrain query ") || !strings.Contains(help, "-limit int") ||
			!strings.Contains(help, "(default 10)") || !strings.Contains(help, "-max-depth") ||
			!strings.Contains(help, "-max-neighbors") || strings.Contains(help, "-weight") {
			t.Fatalf("%v printed unexpected query help:\n%s", args, help)
		}
	}
}

func TestLinkConfidenceFlag(t *testing.T) {
	for input, want := range map[string]*float64{"": nil, "0": new(0.0), "0.25": new(0.25)} {
		var request protocol.LinkRequest
		flags := newFlags("link")
		linkFlags(flags, &request)
		args := []string{"--weight", "2"}
		if input != "" {
			args = append(args, "--confidence", input)
		}
		if err := flags.Parse(args); err != nil {
			t.Fatal(err)
		}
		if request.Weight != 2 || !reflect.DeepEqual(request.Confidence, want) {
			t.Fatalf("parsing %v produced %+v", args, request)
		}
	}
}

func TestConsolidateFlags(t *testing.T) {
	var r protocol.ConsolidateRequest
	flags := newFlags("consolidate")
	consolidateFlags(flags, &r)
	if err := flags.Parse([]string{"--key", "lesson", "--evidence", "a", "--evidence", "b",
		"--text", "retry", "--applies-when", "quota resets", "--rationale", "two trials", "--producer", "agent", "--source", "review"}); err != nil {
		t.Fatal(err)
	}
	want := protocol.ConsolidateRequest{Key: "lesson", EvidenceIDs: []string{"a", "b"},
		ExternalProposal: &protocol.ExternalProposal{Text: "retry", AppliesWhen: "quota resets", Rationale: "two trials", Producer: "agent", Source: "review"}}
	if !reflect.DeepEqual(r, want) {
		t.Fatalf("parsed request = %+v, want %+v", r, want)
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
		{"import needs a directory", []string{"import", "--global", "--json"}, true, "INVALID_REQUEST"},
		{"import takes one directory", []string{"import", "docs", "more", "--json"}, true, "INVALID_REQUEST"},
		{"import scopes conflict across directory", []string{"import", "--project", "A", "docs", "--global", "--json"}, true, "INVALID_SCOPE"},
		{"consolidation is project-local", []string{"consolidate", "--global", "--key", "draft", "--evidence", "a", "--json"}, true, "INVALID_SCOPE"},
		{"empty external draft", []string{"consolidate", "--db", "/dev/null", "--project", "A", "--key", "draft", "--evidence", "a", "--text", "", "--json"}, true, "INVALID_REQUEST"},
		{"JSON after invalid value", []string{"query", "--limit", "many", "--json=T"}, true, "INVALID_REQUEST"},
		{"graph depth on lookup", []string{"query", "--global", "--id", "a", "--max-depth", "1", "--json"}, true, "INVALID_REQUEST"},
		{"graph neighbors on text mode", []string{"query", "--global", "--text", "a", "--mode", "text", "--max-neighbors", "1", "--json"}, true, "INVALID_REQUEST"},
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

func TestGlobalWeightReviewRejectsProjectSelectionOutsideGit(t *testing.T) {
	t.Chdir(t.TempDir())
	var stdout, stderr bytes.Buffer
	err := run([]string{"review-weight", "--current-project", "--json"}, &stdout, &stderr)
	var response protocol.Response
	if jsonErr := json.Unmarshal(stdout.Bytes(), &response); jsonErr != nil {
		t.Fatal(jsonErr)
	}
	if err == nil || response.Error == nil || response.Error.Code != "INVALID_SCOPE" || stderr.Len() != 0 {
		t.Fatalf("expected a JSON scope error, got %+v, err %v, stderr %q", response, err, &stderr)
	}
}

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

func TestPluginBindingWorktrees(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "main")
	other := filepath.Join(base, "worktree")
	git := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	git("init", "-q", root)
	git("-C", root, "-c", "user.name=fixture", "-c", "user.email=fixture@localhost", "commit", "--allow-empty", "--no-gpg-sign", "-qm", "fixture")
	git("-C", root, "worktree", "add", "--detach", other)
	if err := os.WriteFile(filepath.Join(root, ".git", "bigbrain-project"), []byte("A\n"), 0600); err != nil {
		t.Fatal(err)
	}
	first := pluginOptions{cwd: root, include: true, session: "same"}
	a, err := first.bind(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	first.cwd = other
	b, err := first.bind(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	if a.Scope != "project:A" || a.Scope != b.Scope || a.Workspace == b.Workspace || !b.IncludeGlobal {
		t.Fatal(a, b)
	}
	first.global = true
	g, err := first.bind(context.Background(), base)
	if err != nil || g.Scope != "global" || g.IncludeGlobal {
		t.Fatal(g, err)
	}
	first.cwd = base
	if _, err := first.bind(context.Background(), base); err == nil {
		t.Fatal("accepted a missing binding")
	}
}

func TestContextBudgets(t *testing.T) {
	var matches []protocol.Match
	for i := range 8 {
		m := protocol.Match{Memory: protocol.Memory{ID: fmt.Sprint(i), Scope: "project:A", Source: strings.Repeat("線", 500), Text: strings.Repeat("語", 600)}}
		matches = append(matches, m, m)
	}
	p := projectContext(matches, "", nil)
	if !utf8.ValidString(p.Text) || len(p.Text) > 8192 || len(p.Receipt.Memories) != 4 || p.Receipt.Duplicates == 0 || p.Receipt.BudgetSkipped == 0 || !strings.Contains(p.Text, "…") {
		t.Fatal(p.Receipt)
	}
	for i := range matches {
		matches[i].Text = "short"
		matches[i].Source = "source"
	}
	if n := len(projectContext(matches, "", nil).Receipt.Memories); n != 5 {
		t.Fatalf("short records: got %d", n)
	}
}

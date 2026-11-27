package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
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

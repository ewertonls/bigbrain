package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func TestWalkMarkdownFiles(t *testing.T) {
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("git init: %s: %v", out, err)
	}
	repo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"docs/guide/intro.MarkDown", "docs/README.MD", "docs/z.md",
		"docs/readme.txt", "docs/.draft.md", "docs/.hidden/private.md",
		"docs/guide/.git/private.md",
	} {
		path := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	for link, target := range map[string]string{"alias.md": "README.MD", "linked-dir": "guide"} {
		if err := os.Symlink(target, filepath.Join(repo, "docs", link)); err != nil {
			t.Fatal(err)
		}
	}

	// The same docs can be reached by an absolute path, by an alias, or from inside the project.
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(repo, alias); err != nil {
		t.Fatal(err)
	}
	want := []string{"docs/README.MD", "docs/guide/intro.MarkDown", "docs/z.md"}
	for _, dir := range []string{filepath.Join(repo, "docs"), filepath.Join(alias, "docs"), "docs"} {
		t.Run(dir, func(t *testing.T) {
			if dir == "docs" {
				t.Chdir(repo)
			}
			var sources []string
			err := walkMarkdownFiles(dir, true, func(path, source string) error {
				if path != filepath.Join(repo, source) {
					t.Errorf("path = %q for source %q", path, source)
				}
				sources = append(sources, source)
				return nil
			})
			if err != nil || !slices.Equal(sources, want) {
				t.Fatalf("sources = %q, err = %v, want %q", sources, err, want)
			}
		})
	}
	var globalSources []string
	err = walkMarkdownFiles(filepath.Join(alias, "docs"), false, func(_, source string) error {
		globalSources = append(globalSources, source)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for i, source := range want {
		want[i] = filepath.ToSlash(filepath.Join(repo, source))
	}
	if !slices.Equal(globalSources, want) {
		t.Errorf("global sources = %q, want %q", globalSources, want)
	}
}

func TestWalkMarkdownFilesOutsideGit(t *testing.T) {
	dir := t.TempDir()
	dir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "docs.md")
	if err := os.WriteFile(file, nil, 0644); err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("stop importing")
	err = walkMarkdownFiles(dir, true, func(path, source string) error {
		if path != file || source != filepath.ToSlash(file) {
			t.Errorf("got path %q and source %q, want %q", path, source, file)
		}
		return wantErr
	})
	if err != wantErr {
		t.Fatalf("got %v, want callback error", err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"", file, link, filepath.Join(dir, "missing")} {
		if err := walkMarkdownFiles(invalid, false, nil); err == nil {
			t.Errorf("expected an error for %q", invalid)
		}
	}
}

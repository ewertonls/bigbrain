package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func walkMarkdownFiles(dir string, projectScope bool, visit func(path, source string) error) error {
	if dir == "" {
		return fmt.Errorf("expected a directory to import")
	}
	root, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}

	var worktree string
	if projectScope {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		out, gitErr := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "--show-toplevel").Output()
		if ctx.Err() != nil {
			return fmt.Errorf("locate Git worktree for %s: %w", dir, ctx.Err())
		}
		if gitErr == nil {
			worktree = strings.TrimSuffix(string(out), "\n")
		}
	}
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != root && strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(entry.Name())) {
		case ".md", ".markdown":
			source := path
			if worktree != "" {
				source, err = filepath.Rel(worktree, path)
				if err != nil {
					return err
				}
			}
			return visit(path, filepath.ToSlash(source))
		}
		return nil
	})
}

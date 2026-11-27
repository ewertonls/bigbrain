package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

const localProjectFile = ".bigbrain-project"

var (
	errProjectUnavailable   = protocol.NewError("PROJECT_UNAVAILABLE", "Run bigbrain project init ID in the project directory, or supply --project ID explicitly.")
	errProjectNotConfigured = protocol.NewError("PROJECT_NOT_CONFIGURED", "Run bigbrain project init ID in the project directory.")
)

func projectOnlyCommand(name string) bool {
	switch name {
	case "consolidate", "publish", "feedback", "forget", "reconcile":
		return true
	}
	return false
}

func (s *scopeOptions) readGlobalFlag(flags *flag.FlagSet, dest *bool) {
	s.readGlobal = dest
	flags.BoolVar(dest, "include-global", false, "include global records (enabled when scope is omitted)")
}

func (s scopeOptions) resolve(flags *flag.FlagSet) (string, error) {
	supplied := make(map[string]bool)
	flags.Visit(func(f *flag.Flag) { supplied[f.Name] = true })
	projectFlag := supplied["project"]
	implicit := !projectFlag && !supplied["current-project"] && !supplied["global"]
	if flags.Name() == "review-weight" && (!s.global || projectFlag || s.current) {
		return "", protocol.NewError("INVALID_SCOPE", "Choose --global for a reviewed global weight change.")
	}
	if implicit || (s.current && !projectFlag && !s.global) {
		scope, err := configuredProjectScope()
		if err != nil {
			return "", err
		}
		if implicit && !supplied["include-global"] && s.readGlobal != nil {
			*s.readGlobal = true
		}
		return scope, nil
	}
	if projectFlag && !s.current && !s.global && protocol.ValidProjectID(s.project) {
		return "project:" + s.project, nil
	}
	if projectOnlyCommand(flags.Name()) {
		return "", protocol.NewError("INVALID_SCOPE", "Choose --project ID or --current-project for "+flags.Name()+".")
	}
	if s.global && !projectFlag && !s.current {
		return "global", nil
	}
	return "", protocol.NewError("INVALID_SCOPE", "Choose exactly one of --global, --current-project, or --project with a valid project ID.")
}

func initProject(id string) (any, error) {
	if !protocol.ValidProjectID(id) {
		return nil, invalid("Run bigbrain project init ID with a project ID of up to 128 ASCII letters, digits, dots, underscores, or hyphens, starting with a letter or digit.")
	}
	path, err := gitProjectFile(context.Background(), ".")
	if err != nil {
		return nil, err
	}
	if path == "" {
		path = localProjectFile
	}
	if err := writeProject(path, id); err != nil {
		return nil, err
	}
	return projectResult{ID: id}, nil
}

func configuredProjectScope() (string, error) {
	path, _, err := projectConfig(context.Background(), ".")
	if err != nil {
		return "", err
	}
	id, err := readProject(path)
	if err != nil {
		return "", err
	}
	return "project:" + id, nil
}

// An empty path selects directory-based configuration outside Git.
func projectConfig(ctx context.Context, cwd string) (string, bool, error) {
	path, err := gitProjectFile(ctx, cwd)
	if err != nil || path != "" {
		return path, true, err
	}
	path, err = findAncestorFile(cwd, localProjectFile)
	if err == nil && path == "" {
		err = errProjectUnavailable
	}
	return path, false, err
}

func gitProjectFile(ctx context.Context, cwd string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	output, err := cmd.Output()
	if errors.Is(err, exec.ErrNotFound) {
		marker, err := findAncestorFile(cwd, ".git")
		if err != nil {
			return "", err
		}
		if marker != "" || os.Getenv("GIT_DIR") != "" || os.Getenv("GIT_WORK_TREE") != "" {
			return "", protocol.NewError("PROJECT_UNAVAILABLE", "Install Git to resolve this worktree, or supply --project ID explicitly.")
		}
		return "", nil
	}
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			for line := range bytes.Lines(exit.Stderr) {
				if bytes.HasPrefix(line, []byte("fatal: not a git repository (")) {
					return "", nil
				}
			}
		}
		return "", protocol.NewError("PROJECT_UNAVAILABLE", "Cannot locate the Git directory. Check the repository or supply --project ID explicitly.")
	}
	dir := string(bytes.TrimSuffix(output, []byte("\n")))
	if !filepath.IsAbs(dir) {
		return "", protocol.NewError("PROJECT_UNAVAILABLE", "Git did not return an absolute common directory.")
	}
	return filepath.Join(dir, "bigbrain-project"), nil
}

func findAncestorFile(cwd, name string) (string, error) {
	dir, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	for {
		path := filepath.Join(dir, name)
		if _, err := os.Lstat(path); err == nil {
			return path, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", projectConfigError(path)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}

func readProject(path string) (string, error) {
	// Opening a FIFO normally blocks before we can check its type.
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if errors.Is(err, os.ErrNotExist) {
		if _, linkErr := os.Lstat(path); !errors.Is(linkErr, os.ErrNotExist) {
			return "", projectConfigError(path)
		}
		return "", errProjectNotConfigured
	}
	if err != nil {
		return "", projectConfigError(path)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return "", projectConfigError(path)
	}
	value, err := io.ReadAll(io.LimitReader(file, 130))
	id := strings.TrimSuffix(string(value), "\n")
	if err != nil || !protocol.ValidProjectID(id) {
		return "", projectConfigError(path)
	}
	return id, nil
}

func writeProject(path, id string) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".bigbrain-project-")
	if err != nil {
		return projectConfigError(path)
	}
	defer os.Remove(file.Name())
	_, writeErr := io.WriteString(file, id+"\n")
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		return projectConfigError(path)
	}

	// Publish a complete file without replacing an existing choice.
	err = os.Link(file.Name(), path)
	if errors.Is(err, os.ErrExist) {
		saved, err := readProject(path)
		if err != nil {
			return err
		}
		if id != saved {
			return protocol.NewError("PROJECT_ALREADY_CONFIGURED", "This directory is already configured with a different project ID.")
		}
		return nil
	}
	if err != nil {
		return projectConfigError(path)
	}
	return nil
}

func projectConfigError(path string) error {
	return protocol.NewError("PROJECT_CONFIG_ERROR", "Check the project file "+path+" and its permissions.")
}

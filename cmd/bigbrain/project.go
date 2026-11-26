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
	"time"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

var (
	errProjectUnavailable   = protocol.NewError("PROJECT_UNAVAILABLE", "Run this command in a Git worktree, or supply --project ID explicitly.")
	errProjectNotConfigured = protocol.NewError("PROJECT_NOT_CONFIGURED", "Run bigbrain project init ID in this worktree.")
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
	path, err := projectFile()
	if err != nil {
		return nil, err
	}
	if err := writeProject(path, id); err != nil {
		return nil, err
	}
	return projectResult{ID: id}, nil
}

func configuredProjectScope() (string, error) {
	path, err := projectFile()
	if err != nil {
		return "", err
	}
	id, err := readProject(path)
	if err != nil {
		return "", err
	}
	return "project:" + id, nil
}

func projectFile() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	output, err := cmd.Output()
	if err != nil {
		return "", errProjectUnavailable
	}
	dir := string(bytes.TrimSuffix(output, []byte("\n")))
	if !filepath.IsAbs(dir) {
		return "", protocol.NewError("PROJECT_UNAVAILABLE", "Git did not return an absolute common directory.")
	}
	return filepath.Join(dir, "bigbrain-project"), nil
}

func readProject(path string) (string, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", errProjectNotConfigured
	}
	if err != nil {
		return "", projectConfigError()
	}
	defer file.Close()
	value, err := io.ReadAll(io.LimitReader(file, 130))
	id := strings.TrimSuffix(string(value), "\n")
	if err != nil || !protocol.ValidProjectID(id) {
		return "", projectConfigError()
	}
	return id, nil
}

func writeProject(path, id string) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".bigbrain-project-")
	if err != nil {
		return projectConfigError()
	}
	defer os.Remove(file.Name())
	_, writeErr := io.WriteString(file, id+"\n")
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		return projectConfigError()
	}

	// Publish a complete file without replacing an existing choice.
	err = os.Link(file.Name(), path)
	if errors.Is(err, os.ErrExist) {
		saved, err := readProject(path)
		if err != nil {
			return err
		}
		if id != saved {
			return protocol.NewError("PROJECT_ALREADY_CONFIGURED", "This Git repository already has a different project ID.")
		}
		return nil
	}
	if err != nil {
		return projectConfigError()
	}
	return nil
}

func projectConfigError() error {
	return protocol.NewError("PROJECT_CONFIG_ERROR", "Check the bigbrain-project file in the Git common directory and its permissions.")
}

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

func (s scopeOptions) resolve(flags *flag.FlagSet) (string, error) {
	explicit := false
	flags.Visit(func(f *flag.Flag) {
		explicit = explicit || f.Name == "project"
	})
	if explicit && (s.current || s.global) || s.current && s.global {
		return "", protocol.NewError("INVALID_SCOPE", "Choose exactly one of --project, --current-project, or --global.")
	}
	if s.global {
		return "global", nil
	}
	if s.current {
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
	if explicit && protocol.ValidProjectID(s.project) {
		return "project:" + s.project, nil
	}
	return "", protocol.NewError("INVALID_SCOPE", "Choose --global, --current-project, or --project with a valid project ID.")
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

func projectFile() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	output, err := cmd.Output()
	if err != nil {
		return "", protocol.NewError("PROJECT_UNAVAILABLE", "Run this command in a Git worktree, or supply --project ID explicitly.")
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
		return "", protocol.NewError("PROJECT_NOT_CONFIGURED", "Run bigbrain project init ID in this worktree.")
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

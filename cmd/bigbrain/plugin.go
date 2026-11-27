package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ewertonls/bigbrain/internal/daemon"
	"github.com/ewertonls/bigbrain/internal/protocol"
)

type binding struct {
	Database      string `json:"database_directory"`
	Scope         string `json:"scope"`
	Workspace     string `json:"workspace_directory"`
	Session       string `json:"session_id"`
	Agent         string `json:"agent_id,omitempty"`
	IncludeGlobal bool   `json:"include_global"`
}

func stateKey(parts ...string) string {
	data, _ := json.Marshal(parts)
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

type pluginOptions struct {
	cwd, session, agent, state string
	global, include            bool
}

func (p *pluginOptions) register(f *flag.FlagSet) {
	f.StringVar(&p.cwd, "cwd", ".", "workspace directory")
	f.StringVar(&p.session, "session", os.Getenv("CODEX_THREAD_ID"), "session ID")
	f.StringVar(&p.agent, "agent", "", "agent ID")
	f.StringVar(&p.state, "state-dir", os.Getenv("BIGBRAIN_CODEX_STATE"), "private helper state directory")
	f.BoolVar(&p.global, "global", false, "read global memories only")
	f.BoolVar(&p.include, "include-global", true, "include global memories in project reads")
}
func (p pluginOptions) bind(ctx context.Context, db string) (binding, error) {
	cwd, err := filepath.Abs(p.cwd)
	if err != nil {
		return binding{}, err
	}
	cwd, err = filepath.EvalSymlinks(cwd)
	if err != nil {
		return binding{}, err
	}
	path, gitRepo, err := projectConfig(ctx, cwd)
	if err != nil {
		return binding{}, err
	}
	id, err := readProject(path)
	if err != nil {
		return binding{}, err
	}
	workspace := filepath.Dir(path)
	if gitRepo {
		cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
		cmd.Dir = cwd
		out, err := cmd.Output()
		if err != nil {
			return binding{}, err
		}
		workspace = strings.TrimSuffix(string(out), "\n")
	}
	if db != "" && !filepath.IsAbs(db) {
		db = filepath.Join(cwd, db)
	}
	loc, err := locateDatabase("project:"+id, db)
	if err != nil {
		return binding{}, err
	}
	b := binding{Database: loc.Directory, Scope: loc.Scope, Workspace: workspace, Session: p.session, Agent: p.agent, IncludeGlobal: p.include}
	if p.global {
		b.Scope = "global"
		b.IncludeGlobal = false
	}
	return b, nil
}
func (p pluginOptions) stateDir() (string, error) {
	dir := p.state
	if dir == "" {
		base := os.Getenv("XDG_STATE_HOME")
		if !filepath.IsAbs(base) {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".local", "state")
		}
		dir = filepath.Join(base, "bigbrain", "codex")
	}
	if !filepath.IsAbs(dir) {
		return "", invalid("Use an absolute state directory.")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return "", invalid("The plugin state directory must be private (0700).")
	}
	return dir, nil
}
func pluginCommand(o *options, args []string) (any, error) {
	if len(args) == 0 || args[0] != "doctor" {
		return nil, invalid("Run bigbrain plugin doctor.")
	}
	f := newFlags("plugin doctor")
	o.register(f)
	var p pluginOptions
	p.register(f)
	if err := o.parseFlags(f, args[1:]); err != nil {
		return parseError(f, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	b, err := p.bind(ctx, o.db)
	if err != nil {
		return nil, err
	}
	dir, err := p.stateDir()
	if err != nil {
		return nil, err
	}
	client, err := daemon.Connect(ctx, b.Database, false)
	if err != nil {
		return nil, err
	}
	status := protocol.StatusResult{State: "stopped"}
	if client != nil {
		status = client.Status()
	}
	return map[string]any{"binding": b, "state_directory": dir, "session_key": stateKey(b.Database, b.Scope, b.Workspace, b.Session, b.Agent), "daemon": status, "native_runtime": "loaded", "host_hooks": "requires local host trust", "viewer": "local browser"}, nil
}

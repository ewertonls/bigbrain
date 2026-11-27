package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
)

type sessionPolicy struct {
	Paused           bool `json:"paused"`
	DisableStartup   bool `json:"disable_startup"`
	DisableCompact   bool `json:"disable_compact"`
	ProjectOnly      bool `json:"project_only"`
	DisableBootstrap bool `json:"disable_bootstrap"`
}
type hookEvent struct {
	Session string `json:"session_id"`
	Agent   string `json:"agent_id"`
	Cwd     string `json:"cwd"`
	Event   string `json:"hook_event_name"`
	Source  string `json:"source"`
}

var errSkip = errors.New("hook skipped")

func runPluginHook(db string, in io.Reader, out io.Writer) error {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	response := map[string]any{}
	// A slow or malformed host input must not hold up the session. This command
	// handles one event and exits, releasing a blocked stdin reader.
	eventReady := make(chan hookEvent, 1)
	go func() {
		var e hookEvent
		data, err := io.ReadAll(io.LimitReader(in, 65537))
		if err == nil && len(data) <= 65536 {
			if json.Unmarshal(data, &e) != nil {
				e = hookEvent{}
			}
		}
		eventReady <- e
	}()
	select {
	case e := <-eventReady:
		text := hookRecall(ctx, start, db, e)
		if text != "" && ctx.Err() == nil {
			response["hookSpecificOutput"] = map[string]string{"hookEventName": "SessionStart", "additionalContext": text}
		}
	case <-ctx.Done():
	}
	return json.NewEncoder(out).Encode(response)
}
func hookRecall(ctx context.Context, start time.Time, db string, e hookEvent) string {
	if e.Event != "SessionStart" || (e.Source != "startup" && e.Source != "resume" && e.Source != "compact") || e.Session == "" || !filepath.IsAbs(e.Cwd) {
		return ""
	}
	p := pluginOptions{cwd: e.Cwd, session: e.Session, agent: e.Agent, include: true, state: os.Getenv("BIGBRAIN_CODEX_STATE")}
	b, err := p.bind(ctx, db)
	if err != nil {
		return ""
	}
	dir, err := p.stateDir()
	if err != nil {
		return ""
	}
	policy, err := readState[sessionPolicy](b.sessionFile(dir, "policy"))
	if err != nil || policy.Paused || (e.Source == "compact" && policy.DisableCompact) || (e.Source != "compact" && policy.DisableStartup) {
		return ""
	}
	b.IncludeGlobal = !policy.ProjectOnly
	path := b.sessionFile(dir, "recall")
	old, err := readState[recallState](path)
	if err != nil {
		return ""
	}
	input := recallInput{Bootstrap: !policy.DisableBootstrap}
	if old.Last != nil {
		for _, m := range old.Last.Memories {
			input.IDs = append(input.IDs, m.ID)
		}
	}
	matches, fallback, err := recallMatches(ctx, b, dir, input, false)
	if err != nil {
		return ""
	}
	var packet contextPacket
	err = updateState(ctx, path, func(s *recallState) error {
		// Coalesce concurrent deliveries; the host does not supply an event ID.
		if s.Source == e.Source && (e.Source == "startup" || start.Sub(s.EpochStart) < 250*time.Millisecond) && s.IncludeGlobal == b.IncludeGlobal {
			seen := map[string]bool{}
			if s.Last != nil {
				for _, m := range s.Last.Memories {
					seen[m.Hash] = true
				}
			}
			packet = projectContext(matches, fallback, seen)
			if packet.Text == "" {
				return errSkip
			}
		} else {
			packet = projectContext(matches, fallback, nil)
		}
		s.Source = e.Source
		s.EpochStart = start
		s.IncludeGlobal = b.IncludeGlobal
		s.Last = &packet.Receipt
		return nil
	})
	if err != nil {
		return ""
	}
	return packet.Text
}

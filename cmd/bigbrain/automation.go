package main

import (
	"context"
	"net/http"
	"time"
)

type observedHooks map[string]time.Time

func (v *viewer) automation(ctx context.Context, b binding, r *http.Request) (any, error) {
	if b.Session == "" {
		return nil, invalid("Open the viewer with a session ID to inspect automation.")
	}
	path := b.sessionFile(v.dir, "policy")
	if r.URL.Path == "/api/policy" {
		var req struct {
			Setting string `json:"setting"`
			Enabled bool   `json:"enabled"`
		}
		if err := decodeBody(r, &req); err != nil {
			return nil, err
		}
		err := updateState(ctx, path, func(p *sessionPolicy) error {
			switch req.Setting {
			case "paused":
				p.Paused = req.Enabled
			case "startup":
				p.DisableStartup = !req.Enabled
			case "compact":
				p.DisableCompact = !req.Enabled
			case "global":
				p.ProjectOnly = !req.Enabled
			case "bootstrap":
				p.DisableBootstrap = !req.Enabled
			default:
				return invalid("Unknown session policy.")
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	policy, err := readState[sessionPolicy](path)
	if err != nil {
		return nil, err
	}
	s, err := readState[recallState](b.sessionFile(v.dir, "recall"))
	if err != nil {
		return nil, err
	}
	if r.URL.Path == "/api/preview" {
		b.IncludeGlobal = !policy.ProjectOnly
		in := recallInput{Bootstrap: !policy.DisableBootstrap}
		if s.Last != nil {
			for _, m := range s.Last.Memories {
				in.IDs = append(in.IDs, m.ID)
			}
		}
		matches, fallback, err := recallMatches(ctx, b, v.dir, in, true)
		if err != nil {
			return nil, err
		}
		return projectContext(matches, fallback, nil), nil
	}
	hooks, err := readState[observedHooks](b.sessionFile(v.dir, "hooks"))
	if err != nil {
		return nil, err
	}
	return map[string]any{"policy": policy, "last": s.Last, "hooks": hooks}, nil
}

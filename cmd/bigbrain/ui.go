package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/ewertonls/bigbrain/internal/daemon"
	"github.com/ewertonls/bigbrain/internal/protocol"
)

//go:embed web/*
var viewerFiles embed.FS

// These templates compose bundled HTML, CSS and JS. Runtime data arrives via the API.
var viewerPages = template.Must(template.New("viewer").Funcs(template.FuncMap{
	"asset": func(name string) (string, error) {
		data, err := viewerFiles.ReadFile("web/" + name)
		return string(data), err
	},
}).ParseFS(viewerFiles, "web/index.html", "web/*.tmpl"))

type viewer struct {
	binding          binding
	pinBinding       sync.Once
	dir, host, token string
	workspace        pluginOptions
	db               string
}

func uiCommand(o *options, args []string) (any, error) {
	f := newFlags("ui")
	o.register(f)
	connectOnly := f.Bool("connect-only", false, "use a running daemon without starting one")
	var p pluginOptions
	p.register(f)
	if err := o.parseFlags(f, args); err != nil {
		return parseError(f, err)
	}
	if p.global {
		return nil, invalid("Launch the viewer from a bound project; select global in Explorer.")
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
	return streamOutput(func(out io.Writer) error {
		listener, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			return err
		}
		defer listener.Close()
		v := viewer{binding: b, dir: dir, host: listener.Addr().String(), token: rand.Text(), workspace: p, db: o.db}
		if _, err = fmt.Fprintf(out, "http://%s/#%s\n", v.host, v.token); err != nil {
			return err
		}
		srv := http.Server{Handler: &v, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 35 * time.Second, IdleTimeout: time.Minute, MaxHeaderBytes: 8192}
		if *connectOnly {
			srv.BaseContext = func(net.Listener) context.Context { return daemon.ConnectOnly(context.Background()) }
		}
		return srv.Serve(listener)
	}), nil
}

func (v *viewer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Cache-Control", "no-store")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
	if r.Host != v.host {
		http.Error(w, "Unexpected host.", 403)
		return
	}
	path := r.URL.Path
	if !strings.HasPrefix(path, "/api/") {
		if r.Method != "GET" {
			http.Error(w, "Expected GET.", 405)
			return
		}
		if path == "/" {
			path = "/index.html"
		}
		if strings.Contains(path[1:], "/") {
			http.NotFound(w, r)
			return
		}
		if path == "/index.html" {
			h.Set("Content-Type", "text/html; charset=utf-8")
			_ = viewerPages.ExecuteTemplate(w, "index.html", nil)
			return
		}
		data, err := viewerFiles.ReadFile("web" + path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		switch {
		case strings.HasSuffix(path, ".js"):
			h.Set("Content-Type", "text/javascript; charset=utf-8")
		case strings.HasSuffix(path, ".css"):
			h.Set("Content-Type", "text/css")
		default:
			h.Set("Content-Type", "text/html; charset=utf-8")
		}
		w.Write(data)
		return
	}
	origin := r.Header.Values("Origin")
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+v.token)) != 1 || len(origin) > 1 || (len(origin) == 1 && origin[0] != "http://"+v.host) || (r.Method != "GET" && len(origin) != 1) {
		http.Error(w, "Viewer authorization expired or origin rejected. Reopen the printed URL.", 403)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, draftBytes+4096)
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	result, err := v.api(ctx, r)
	h.Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
	}
	_ = protocol.Encode(w, result, err)
}

func decodeBody(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if dec.Decode(new(any)) != io.EOF {
		return errors.New("Expected one JSON request.")
	}
	return nil
}

func (v *viewer) api(ctx context.Context, r *http.Request) (any, error) {
	if r.Method == "GET" && r.URL.Path == "/api/status" {
		state, err := lifecycle("status", v.binding.Database)
		if err != nil {
			return nil, err
		}
		return map[string]any{"binding": v.binding, "daemon": state}, nil
	}
	if r.Method != "POST" {
		return nil, invalid("Unsupported viewer operation.")
	}
	if err := v.checkBinding(ctx); err != nil {
		return nil, err
	}
	b := v.binding
	switch r.URL.Path {
	case "/api/automation", "/api/preview", "/api/policy":
		return v.automation(ctx, b, r)
	case "/api/drafts", "/api/edit", "/api/save", "/api/dismiss":
		if b.Session == "" {
			return nil, invalid("Reopen with --session SESSION to review handoffs.")
		}
		path := b.sessionFile(v.dir, "drafts")
		if r.URL.Path == "/api/drafts" {
			ds, err := readState[drafts](path)
			if ds == nil {
				ds = drafts{}
			}
			return ds, err
		}
		var req draftAction
		if err := decodeBody(r, &req); err != nil {
			return nil, err
		}
		return v.applyDraft(ctx, strings.TrimPrefix(r.URL.Path, "/api/"), req)
	case "/api/browse":
		var q protocol.BrowseRequest
		if err := decodeBody(r, &q); err != nil {
			return nil, err
		}
		return v.browse(ctx, q)
	case "/api/query":
		var q protocol.QueryRequest
		if err := decodeBody(r, &q); err != nil {
			return nil, err
		}
		return v.query(ctx, q)
	case "/api/feedback":
		var q protocol.FeedbackHistoryRequest
		if err := decodeBody(r, &q); err != nil {
			return nil, err
		}
		return v.feedbackHistory(ctx, q)
	case "/api/history":
		var q protocol.HistoryRequest
		if err := decodeBody(r, &q); err != nil {
			return nil, err
		}
		return v.history(ctx, q)
	default:
		return nil, invalid("Unsupported viewer operation.")
	}
}

func (v *viewer) checkBinding(ctx context.Context) error {
	b, err := v.workspace.bind(ctx, v.db)
	if err != nil {
		return err
	}
	v.pinBinding.Do(func() {
		if v.binding.Scope == "" {
			v.binding = b
		}
	})
	if b != v.binding {
		return invalid("Workspace binding changed. Reopen the viewer.")
	}
	return nil
}

func (v *viewer) query(ctx context.Context, q protocol.QueryRequest) (protocol.QueryResult, error) {
	return readVisible[protocol.QueryResult](ctx, v, q.Scope, "query", q)
}

func (v *viewer) history(ctx context.Context, q protocol.HistoryRequest) (protocol.HistoryResult, error) {
	return readVisible[protocol.HistoryResult](ctx, v, q.Scope, "history", q)
}

func (v *viewer) browse(ctx context.Context, q protocol.BrowseRequest) (protocol.BrowseResult, error) {
	return readVisible[protocol.BrowseResult](ctx, v, q.Scope, "browse", q)
}

type draftAction struct {
	ID       string       `json:"id"`
	Revision int          `json:"revision"`
	Content  draftContent `json:"content,omitempty"`
}

func (v *viewer) applyDraft(ctx context.Context, op string, req draftAction) (handoffDraft, error) {
	b := v.binding
	if op == "save" {
		return saveHandoff(ctx, b, v.dir, req.ID, req.Revision, true)
	}
	return changeDraft(ctx, b.sessionFile(v.dir, "drafts"), b, op, req.ID, req.Revision, req.Content)
}

func (v *viewer) feedbackHistory(ctx context.Context, q protocol.FeedbackHistoryRequest) (protocol.FeedbackHistoryResult, error) {
	return readVisible[protocol.FeedbackHistoryResult](ctx, v, q.Scope, "feedback-history", q)
}

// Both transports must validate the same binding before contacting the daemon.
func readVisible[R any](ctx context.Context, v *viewer, scope, operation string, request interface{ Validate() error }) (R, error) {
	var zero R
	if scope != v.binding.Scope && scope != "global" {
		return zero, invalid("Scope outside this viewer.")
	}
	if err := request.Validate(); err != nil {
		return zero, err
	}
	return callDaemon[R](ctx, v.binding.Database, operation, request)
}

package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ewertonls/bigbrain/internal/protocol"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

//go:embed web/*
var viewerFiles embed.FS

type viewer struct {
	binding          binding
	dir, host, token string
	workspace        pluginOptions
	db               string
}

func uiCommand(o *options, args []string) (any, error) {
	f := newFlags("ui")
	o.register(f)
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
	b, err := v.workspace.bind(ctx, v.db)
	if err != nil {
		return nil, err
	}
	if b != v.binding {
		return nil, invalid("Workspace binding changed. Reopen the viewer.")
	}
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
		var req struct {
			ID       string       `json:"id"`
			Revision int          `json:"revision"`
			Content  draftContent `json:"content"`
		}
		if err := decodeBody(r, &req); err != nil {
			return nil, err
		}
		if r.URL.Path == "/api/save" {
			return saveHandoff(ctx, b, v.dir, req.ID, req.Revision, true)
		}
		return changeDraft(ctx, path, b, strings.TrimPrefix(r.URL.Path, "/api/"), req.ID, req.Revision, req.Content)
	case "/api/query":
		var q protocol.QueryRequest
		if err := decodeBody(r, &q); err != nil {
			return nil, err
		}
		if q.Scope != b.Scope && q.Scope != "global" {
			return nil, invalid("Scope outside this viewer.")
		}
		if err := q.Validate(); err != nil {
			return nil, err
		}
		return callDaemon[protocol.QueryResult](ctx, b.Database, "query", q)
	case "/api/history":
		var q protocol.HistoryRequest
		if err := decodeBody(r, &q); err != nil {
			return nil, err
		}
		if q.Scope != b.Scope && q.Scope != "global" {
			return nil, invalid("Scope outside this viewer.")
		}
		if err := q.Validate(); err != nil {
			return nil, err
		}
		return callDaemon[protocol.HistoryResult](ctx, b.Database, "history", q)
	default:
		return nil, invalid("Unsupported viewer operation.")
	}
}

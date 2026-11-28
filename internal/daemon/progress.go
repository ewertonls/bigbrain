package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptrace"
	"net/textproto"
	"time"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

const progressHeader = "X-BigBrain-Progress"

type queryProgressKey struct{}

func WithQueryProgress(ctx context.Context, report func(protocol.IndexProgress)) context.Context {
	if report == nil {
		return ctx
	}
	return context.WithValue(ctx, queryProgressKey{}, report)
}

// Opted-in clients receive HTTP 102 Processing responses before the usual
// v1 JSON envelope. Older clients and service callers see only the envelope.
func traceQueryProgress(req *http.Request) *http.Request {
	report, _ := req.Context().Value(queryProgressKey{}).(func(protocol.IndexProgress))
	if report == nil {
		return req
	}
	req.Header.Set(progressHeader, "1")
	trace := &httptrace.ClientTrace{Got1xxResponse: func(code int, hdr textproto.MIMEHeader) error {
		if code != http.StatusProcessing {
			return nil
		}
		var event protocol.IndexProgress
		if err := json.Unmarshal([]byte(hdr.Get(progressHeader)), &event); err != nil {
			return err
		}
		report(event)
		return nil
	}}
	return req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
}

func (s *serverState) queryWithProgress(w http.ResponseWriter, r *http.Request, request protocol.QueryRequest) (protocol.QueryResult, error) {
	controller := http.NewResponseController(w)
	// A cold rebuild may exceed the regular response write timeout.
	_ = controller.SetWriteDeadline(time.Time{})
	defer func() { _ = controller.SetWriteDeadline(time.Now().Add(30 * time.Second)) }()
	var report func(protocol.IndexProgress)
	if r.Header.Get(progressHeader) == "1" {
		var last time.Time
		report = func(p protocol.IndexProgress) {
			if p.Completed != 0 && p.Completed != p.Total && time.Since(last) < 80*time.Millisecond {
				return
			}
			last = time.Now()
			event, _ := json.Marshal(p)
			w.Header().Set(progressHeader, string(event))
			_ = controller.SetWriteDeadline(last.Add(5 * time.Second))
			w.WriteHeader(http.StatusProcessing)
			w.Header().Del(progressHeader)
			_ = controller.SetWriteDeadline(time.Time{})
		}
	}
	return s.database.QueryWithProgress(r.Context(), request, report)
}

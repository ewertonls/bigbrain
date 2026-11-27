package main

import (
	"net/http/httptest"
	"testing"
)

func TestViewerRequestBoundary(t *testing.T) {
	root := t.TempDir()
	v := viewer{host: "127.0.0.1:12345", token: "secret", binding: binding{Database: root + "/db"}}
	for _, tc := range []struct {
		method, path, host, token, origin string
		want                              int
	}{
		{"GET", "/", "127.0.0.1:12345", "", "", 200},
		{"GET", "/api/status", "127.0.0.1:12345", "", "", 403},
		{"GET", "/api/status", "127.0.0.1:12345", "bad", "", 403},
		{"GET", "/api/status", "wrong", "secret", "", 403},
		{"GET", "/api/status", "127.0.0.1:12345", "secret", "http://other", 403},
		{"GET", "/api/status", "127.0.0.1:12345", "secret", "", 200},
		{"POST", "/api/query", "127.0.0.1:12345", "secret", "", 403},
	} {
		r := httptest.NewRequest(tc.method, "http://"+tc.host+tc.path, nil)
		if tc.token != "" {
			r.Header.Set("Authorization", "Bearer "+tc.token)
		}
		if tc.origin != "" {
			r.Header.Set("Origin", tc.origin)
		}
		w := httptest.NewRecorder()
		v.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("%s %s %s: %d", tc.method, tc.host, tc.path, w.Code)
		}
	}
}

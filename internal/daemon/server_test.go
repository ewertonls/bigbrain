package daemon

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

func TestConnectOnlyDoesNotStartDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new-db")
	ctx := ConnectOnly(t.Context())
	client, err := Connect(ctx, path, true)
	var unavailable *protocol.Error
	if client != nil || !errors.As(err, &unavailable) || unavailable.Code != "DAEMON_UNAVAILABLE" {
		t.Fatalf("expected a stopped daemon error, got client %v, err %v", client, err)
	}
	if client, err = Connect(ctx, path, false); client != nil || err != nil {
		t.Fatalf("status probe returned client %v, err %v", client, err)
	}
	if _, err = os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("connect-only created the database directory: %v", err)
	}
}

func TestOperationReleasesDatabaseLockAfterPanic(t *testing.T) {
	state := &serverState{identity: protocol.StatusResult{InstanceID: "test"}}
	handler := operation(state, func(protocol.RecordRequest) (protocol.WriteResult, error) {
		panic("store panic")
	})
	body := strings.NewReader(`{"scope":"project:test","key":"one","text":"Memory","source":"Fixture"}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/record", body)
	request.Header.Set(instanceHeader, state.identity.InstanceID)
	request.Header.Set(versionHeader, strconv.Itoa(protocol.Version))

	func() {
		defer func() {
			if recovered := recover(); recovered != "store panic" {
				t.Fatalf("expected store panic, got %v", recovered)
			}
		}()
		handler(httptest.NewRecorder(), request)
	}()

	if !state.databaseMu.TryLock() {
		t.Fatal("database lock is still held after operation panicked")
	}
	state.databaseMu.Unlock()
}

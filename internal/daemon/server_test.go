package daemon

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

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

package daemon

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/ewertonls/bigbrain/internal/protocol"
	"github.com/ewertonls/bigbrain/internal/store"
)

// The acceptance build can install barriers around commit and acknowledgment.
var configureStore = func(*store.Store) {}

func Serve(dbDir string) (err error) {
	ready := func(error) {}
	var lock *os.File
	if os.Getenv(inheritedEnv) == "1" {
		lock = os.NewFile(3, "inherited database lock")
		pipe := os.NewFile(4, "daemon readiness pipe")
		ready = func(startErr error) {
			if pipe == nil {
				return
			}
			_ = protocol.Encode(pipe, struct{}{}, startErr)
			pipe.Close()
			pipe = nil
		}
	}
	defer func() { ready(err) }()

	t, err := databaseTarget(dbDir, true)
	if err != nil {
		if lock != nil {
			lock.Close()
		}
		return protocol.NewError("STARTUP_FAILED", err.Error())
	}
	if lock != nil {
		defer lock.Close()
		info, statErr := lock.Stat()
		expected, dirErr := os.Lstat(t.lockPath())
		if statErr != nil || dirErr != nil || !os.SameFile(info, expected) {
			return protocol.NewError("STARTUP_FAILED", "The inherited ownership lock does not match the database.")
		}
		if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			return protocol.NewError("DATABASE_BUSY", "Another daemon owns this database.")
		}
	} else {
		lock, err = takeLock(t, true)
		if err != nil {
			return protocol.NewError("STARTUP_FAILED", err.Error())
		}
		if lock == nil {
			return protocol.NewError("DATABASE_BUSY", "Another daemon owns this database.")
		}
		defer lock.Close()
	}
	if err := clearStaleSocket(t); err != nil {
		return protocol.NewError("STARTUP_FAILED", err.Error())
	}
	info, err := os.Lstat(t.databasePath())
	if err == nil && (!info.Mode().IsRegular() || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid())) {
		return protocol.NewError("STARTUP_FAILED", "The database must be a regular file owned by this user.")
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return protocol.NewError("STARTUP_FAILED", err.Error())
	}
	database, err := store.Open(t.databasePath())
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			database.Close()
		}
	}()
	configureStore(database)
	state := &serverState{
		database: database,
		stop:     make(chan struct{}),
		identity: protocol.StatusResult{
			Running: true, State: "running", DatabaseID: t.id, InstanceID: rand.Text(), PID: os.Getpid(),
		},
	}
	if err := writeInstance(t, state.identity); err != nil {
		return protocol.NewError("STARTUP_FAILED", err.Error())
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: t.socket, Net: "unix"})
	if err != nil {
		return protocol.NewError("STARTUP_FAILED", err.Error())
	}
	defer listener.Close()
	if err := os.Chmod(t.socket, 0o600); err != nil {
		return protocol.NewError("STARTUP_FAILED", err.Error())
	}
	server := &http.Server{
		Handler:           state.routes(),
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       5 * time.Second,
	}
	serveError := make(chan error, 1)
	go func() { serveError <- server.Serve(listener) }()
	signals, stopSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignal()
	log.Printf("Daemon ready for database %s (instance %s, PID %d)", t.id, state.identity.InstanceID, state.identity.PID)
	ready(nil)
	select {
	case <-state.stop:
	case <-signals.Done():
	case err = <-serveError:
	}
	state.drain()
	log.Printf("Daemon draining accepted requests (instance %s)", state.identity.InstanceID)
	state.inflight.Wait()
	database.Close()
	closed = true
	if shutdownErr := server.Shutdown(context.Background()); err == nil {
		err = shutdownErr
	}
	log.Printf("Daemon stopped (instance %s)", state.identity.InstanceID)
	return err
}

type serverState struct {
	identity protocol.StatusResult
	stop     chan struct{}

	mu       sync.Mutex
	draining bool
	inflight sync.WaitGroup

	// A single connection serializes all work, including read transactions.
	databaseMu sync.Mutex
	database   *store.Store
}

func (s *serverState) drain() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.draining {
		s.draining = true
		close(s.stop)
	}
}

func (s *serverState) beginRequest() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.draining {
		return false
	}
	s.inflight.Add(1)
	return true
}

func (s *serverState) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		status := s.identity
		s.mu.Lock()
		if s.draining {
			status.State = "draining"
		}
		s.mu.Unlock()
		respond(w, status, nil)
	})
	mux.HandleFunc("POST /v1/record", operation(s, s.database.Record))
	mux.HandleFunc("POST /v1/link", operation(s, s.database.Link))
	mux.HandleFunc("POST /v1/query", operation(s, s.database.Query))
	mux.HandleFunc("POST /v1/shutdown", func(w http.ResponseWriter, r *http.Request) {
		if err := s.verifyRequest(r); err != nil {
			respond(w, nil, err)
			return
		}
		s.drain()
		respond(w, struct{}{}, nil)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		respond(w, nil, protocol.NewError("INVALID_REQUEST", "Unknown BigBrain endpoint or HTTP method."))
	})
	return mux
}

func (s *serverState) verifyRequest(r *http.Request) error {
	if r.Header.Get(versionHeader) != strconv.Itoa(protocol.Version) {
		return protocol.NewError("VERSION_MISMATCH", "Use CLI and daemon from the same BigBrain build.")
	}
	if r.Header.Get(instanceHeader) != s.identity.InstanceID {
		return protocol.NewError("INSTANCE_CHANGED", "The daemon instance changed. For a write, retry with the saved request and key.")
	}
	return nil
}

func operation[Request interface{ Validate() error }, Result any](s *serverState, execute func(Request) (Result, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := s.verifyRequest(r); err != nil {
			respond(w, nil, err)
			return
		}
		if !s.beginRequest() {
			respond(w, nil, shuttingDownError())
			return
		}
		defer s.inflight.Done()
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, protocol.MaxBodyBytes))
		decoder.DisallowUnknownFields()
		var request Request
		if err := decoder.Decode(&request); err != nil || decoder.Decode(new(any)) != io.EOF {
			respond(w, nil, protocol.NewError("INVALID_REQUEST", "Expected one valid JSON object for this operation."))
			return
		}
		if err := request.Validate(); err != nil {
			respond(w, nil, err)
			return
		}
		result, err := executeLocked(&s.databaseMu, execute, request)
		respond(w, result, err)
	}
}

func executeLocked[Request, Result any](mu *sync.Mutex, execute func(Request) (Result, error), request Request) (Result, error) {
	mu.Lock()
	defer mu.Unlock()
	return execute(request)
}

func respond(w http.ResponseWriter, result any, err error) {
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		var responseError *protocol.Error
		status := http.StatusInternalServerError
		if errors.As(err, &responseError) {
			switch responseError.Code {
			case "SHUTTING_DOWN":
				status = http.StatusServiceUnavailable
			case "IDEMPOTENCY_CONFLICT", "INSTANCE_CHANGED", "VERSION_MISMATCH":
				status = http.StatusConflict
			default:
				status = http.StatusBadRequest
			}
		} else {
			log.Printf("Database operation failed: %v", err)
		}
		w.WriteHeader(status)
	}
	_ = protocol.Encode(w, result, err)
}

func shuttingDownError() error {
	return protocol.NewError("SHUTTING_DOWN", "The daemon is finishing accepted requests. Retry once shutdown completes.")
}

func readInstance(t target) (protocol.StatusResult, error) {
	var identity protocol.StatusResult
	file, err := openPrivateFile(t.instancePath(), syscall.O_RDONLY)
	if err != nil {
		return identity, err
	}
	defer file.Close()
	err = json.NewDecoder(io.LimitReader(file, 4096)).Decode(&identity)
	return identity, err
}

func writeInstance(t target, identity protocol.StatusResult) error {
	file, err := os.CreateTemp(t.dir, ".daemon-instance-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	err = json.NewEncoder(file).Encode(identity)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(file.Name(), t.instancePath())
}

func clearStaleSocket(t target) error {
	info, err := os.Lstat(t.socket)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	identity, err := readInstance(t)
	if err != nil || identity.DatabaseID != t.id || identity.InstanceID == "" || identity.PID < 1 ||
		info.Mode()&os.ModeSocket == 0 || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
		return fmt.Errorf("cannot verify which instance owns the socket at %s", t.socket)
	}
	conn, err := net.DialTimeout("unix", t.socket, time.Second)
	if err == nil {
		conn.Close()
		return fmt.Errorf("an instance is still reachable at %s", t.socket)
	}
	if !socketAbsent(err) {
		return fmt.Errorf("cannot verify that the socket is stale: %w", err)
	}
	if err := os.Remove(t.socket); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

const (
	instanceHeader = "X-BigBrain-Instance"
	versionHeader  = "X-BigBrain-Version"
	inheritedEnv   = "BIGBRAIN_DAEMON_INHERITED"
	startupLimit   = 10 * time.Second
	pollInterval   = 25 * time.Millisecond
)

type Client struct {
	http   *http.Client
	status protocol.StatusResult
}

type connectOnlyKey struct{}

// ConnectOnly prevents a supervised viewer from starting a shared daemon that
// its host would terminate along with the viewer's process tree.
func ConnectOnly(ctx context.Context) context.Context {
	return context.WithValue(ctx, connectOnlyKey{}, true)
}

func Connect(ctx context.Context, dbDir string, autoStart bool) (*Client, error) {
	connectOnly := ctx.Value(connectOnlyKey{}) == true
	client, err := connect(ctx, dbDir, autoStart && !connectOnly)
	if autoStart && connectOnly && err == nil && client == nil {
		return nil, protocol.NewError("DAEMON_UNAVAILABLE", "The daemon stopped. Reopen the Explorer.")
	}
	return client, err
}

func connect(ctx context.Context, dbDir string, autoStart bool) (*Client, error) {
	t, err := databaseTarget(dbDir, autoStart)
	if err != nil {
		if !autoStart && errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, protocol.NewError("DAEMON_UNAVAILABLE", err.Error())
	}
	if !autoStart {
		return discover(ctx, t)
	}
	ctx, cancel := context.WithTimeout(ctx, startupLimit)
	defer cancel()
	for {
		if ctx.Err() != nil {
			return nil, startupTimeout(t)
		}
		client, err := discover(ctx, t)
		if err != nil || client != nil {
			return client, err
		}
		lock, err := takeLock(t, true)
		if err != nil {
			return nil, protocol.NewError("STARTUP_FAILED", err.Error())
		}
		if lock != nil {
			if err := launch(ctx, t, lock); err != nil {
				return nil, err
			}
			client, err = discover(ctx, t)
			if err != nil || client != nil {
				return client, err
			}
			return nil, protocol.NewError("STARTUP_FAILED", "The daemon exited after signaling readiness. See "+t.logPath()+".")
		}
		if !pause(ctx) {
			return nil, startupTimeout(t)
		}
	}
}

func (c *Client) Status() protocol.StatusResult { return c.status }

func (c *Client) Call(ctx context.Context, op string, request, result any) error {
	if c.status.State == "draining" && op != "shutdown" {
		return shuttingDownError()
	}
	var body io.Reader
	if request != nil {
		data, err := json.Marshal(request)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	err := c.request(ctx, http.MethodPost, "/v1/"+op, body, result)
	var responseError *protocol.Error
	if err != nil && op != "shutdown" && !errors.As(err, &responseError) {
		return protocol.NewError("DAEMON_UNAVAILABLE", "The daemon response was lost. If a write may have been accepted, retry with the saved request and key.")
	}
	return err
}

func discover(ctx context.Context, t target) (*Client, error) {
	dialer := net.Dialer{}
	transport := &http.Transport{
		DisableKeepAlives: true,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", t.socket)
		},
	}
	client := &Client{http: &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}
	probe, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := client.request(probe, http.MethodGet, "/status", nil, &client.status); err != nil {
		if socketAbsent(err) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) ||
			errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, nil
		}
		var responseError *protocol.Error
		if errors.As(err, &responseError) {
			return nil, err
		}
		return nil, protocol.NewError("DAEMON_UNAVAILABLE", "Could not check daemon status: "+err.Error())
	}
	if !client.status.Running || client.status.DatabaseID != t.id || client.status.InstanceID == "" || client.status.PID < 1 {
		return nil, protocol.NewError("DATABASE_MISMATCH", "The socket does not belong to the selected database.")
	}
	identity, err := readInstance(t)
	if err != nil || identity.DatabaseID != t.id || identity.InstanceID != client.status.InstanceID || identity.PID != client.status.PID {
		return nil, protocol.NewError("INSTANCE_CHANGED", "Unable to verify the daemon instance. Try the command again.")
	}
	return client, nil
}

func (c *Client) request(ctx context.Context, method, path string, body io.Reader, result any) error {
	req, err := http.NewRequestWithContext(ctx, method, "http://bigbrain"+path, body)
	if err != nil {
		return err
	}
	req.Header.Set(versionHeader, strconv.Itoa(protocol.Version))
	req.Header.Set(instanceHeader, c.status.InstanceID)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if path == "/v1/query" {
		req = traceQueryProgress(req)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var envelope protocol.Response
	if err := json.NewDecoder(io.LimitReader(resp.Body, protocol.MaxResponseBytes)).Decode(&envelope); err != nil {
		if method == http.MethodGet && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			return protocol.NewError("VERSION_MISMATCH", "The daemon did not return a v1 status. Use CLI and daemon from the same BigBrain build.")
		}
		return err
	}
	if envelope.Version != protocol.Version {
		return protocol.NewError("VERSION_MISMATCH", fmt.Sprintf("Daemon protocol version %d is incompatible with CLI version %d. Use CLI and daemon from the same BigBrain build.", envelope.Version, protocol.Version))
	}
	if envelope.Error != nil {
		return envelope.Error
	}
	if resp.StatusCode != http.StatusOK || envelope.Result == nil {
		return protocol.NewError("INVALID_RESPONSE", "The daemon returned an invalid success response.")
	}
	if result == nil {
		return nil
	}
	return json.Unmarshal(envelope.Result, result)
}

func socketAbsent(err error) bool {
	return errors.Is(err, syscall.ENOENT) || errors.Is(err, syscall.ECONNREFUSED)
}

func launch(ctx context.Context, t target, lock *os.File) error {
	defer lock.Close()
	executable, err := os.Executable()
	if err != nil {
		return protocol.NewError("STARTUP_FAILED", err.Error())
	}
	read, write, err := os.Pipe()
	if err != nil {
		return protocol.NewError("STARTUP_FAILED", err.Error())
	}
	defer read.Close()
	logFile, err := openPrivateFile(t.logPath(), syscall.O_WRONLY|syscall.O_CREAT|syscall.O_APPEND)
	if err != nil {
		write.Close()
		return protocol.NewError("STARTUP_FAILED", err.Error())
	}
	defer logFile.Close()
	cmd := exec.Command(executable, "__daemon", "--db", t.dir)
	cmd.Env = append(os.Environ(), inheritedEnv+"=1")
	cmd.ExtraFiles = []*os.File{lock, write}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		write.Close()
		return protocol.NewError("STARTUP_FAILED", err.Error())
	}
	write.Close()
	go func() { _ = cmd.Wait() }()

	ready := make(chan error, 1)
	go func() { ready <- readReadiness(read, t.logPath()) }()
	select {
	case err := <-ready:
		return err
	case <-ctx.Done():
		// The daemon owns the lock and may still be migrating the database.
		// A later client can connect when initialization finishes.
		return startupTimeout(t)
	}
}

func readReadiness(reader io.Reader, logPath string) error {
	var response protocol.Response
	if err := json.NewDecoder(reader).Decode(&response); err != nil {
		return protocol.NewError("STARTUP_FAILED", "The daemon exited before becoming ready. See "+logPath+".")
	}
	if response.Version != protocol.Version {
		return protocol.NewError("VERSION_MISMATCH", "Use CLI and daemon from the same BigBrain build.")
	}
	if response.Error != nil && response.Result == nil && response.Error.Code != "" && response.Error.Message != "" {
		return response.Error
	}
	if response.Error != nil || !bytes.Equal(response.Result, []byte("{}")) {
		return protocol.NewError("STARTUP_FAILED", "The daemon returned an invalid readiness response. See "+logPath+".")
	}
	return nil
}

func startupTimeout(t target) error {
	return protocol.NewError("STARTUP_TIMEOUT", "The daemon did not become ready before the deadline. See "+t.logPath()+".")
}

func pause(ctx context.Context) bool {
	timer := time.NewTimer(pollInterval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

package daemon

import (
	"context"
	"errors"
	"os"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

func Shutdown(ctx context.Context, dbDir string) error {
	t, err := databaseTarget(dbDir, false)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return protocol.NewError("DAEMON_UNAVAILABLE", err.Error())
	}
	for {
		client, err := discover(ctx, t)
		if err != nil {
			return err
		}
		if client != nil {
			return stopInstance(ctx, t, client)
		}
		held, err := lockHeld(t)
		if err != nil {
			return protocol.NewError("DAEMON_UNAVAILABLE", err.Error())
		}
		if !held {
			return nil
		}
		// A daemon may be gaining readiness or finishing teardown.
		if !pause(ctx) {
			return shutdownTimeout()
		}
	}
}

func stopInstance(ctx context.Context, t target, client *Client) error {
	instance := client.status.InstanceID
	process, err := pinProcess(client.status.PID)
	if err != nil {
		return protocol.NewError("DAEMON_UNAVAILABLE", err.Error())
	}
	if process != nil {
		defer process.Close()
	}
	// Verify that the PID still refers to the process that owns this socket.
	current, err := discover(ctx, t)
	if err != nil {
		return err
	}
	if current == nil || current.status.InstanceID != instance {
		return waitForExit(ctx, t, instance, nil)
	}
	callErr := client.Call(ctx, "shutdown", nil, nil)
	var responseError *protocol.Error
	if errors.As(callErr, &responseError) && responseError.Code != "INSTANCE_CHANGED" {
		return callErr
	}
	// A lost response can be the last observation of the old daemon.
	// Wait for that instance without sending shutdown to a replacement.
	return waitForExit(ctx, t, instance, process)
}

func waitForExit(ctx context.Context, t target, instance string, process *os.File) error {
	for {
		held, err := lockHeld(t)
		if err != nil {
			return protocol.NewError("DAEMON_UNAVAILABLE", err.Error())
		}
		ownershipEnded := !held
		if held {
			identity, err := readInstance(t)
			ownershipEnded = err == nil && identity.InstanceID != instance
		}
		if ownershipEnded {
			exited, err := processExited(process)
			if err != nil {
				return protocol.NewError("DAEMON_UNAVAILABLE", err.Error())
			}
			if exited {
				return nil
			}
		}
		if !pause(ctx) {
			return shutdownTimeout()
		}
	}
}

func shutdownTimeout() error {
	return protocol.NewError("SHUTDOWN_TIMEOUT", "The selected daemon did not exit before the deadline.")
}

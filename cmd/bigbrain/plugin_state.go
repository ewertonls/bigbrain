package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

func (b binding) sessionFile(dir, kind string) string {
	return filepath.Join(dir, kind+"-"+stateKey(b.Database, b.Scope, b.Workspace, b.Session, b.Agent)+".json")
}

func (b binding) handoffPointer(dir string) string {
	return filepath.Join(dir, "latest-"+stateKey(b.Database, b.Scope)+".json")
}

func readState[T any](path string) (T, error) {
	var v T
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return v, nil
	}
	if err != nil {
		return v, err
	}
	err = json.Unmarshal(data, &v)
	return v, err
}

// Never hold this lock across daemon requests. Readers see complete files after rename.
func updateState[T any](ctx context.Context, path string, change func(*T) error) error {
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err = unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, unix.EWOULDBLOCK) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
	v, err := readState[T](path)
	if err != nil {
		return err
	}
	if err = change(&v); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".pending-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	err = json.NewEncoder(f).Encode(v)
	if err == nil {
		err = f.Sync()
	}
	cerr := f.Close()
	if err != nil {
		return err
	}
	if cerr != nil {
		return cerr
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

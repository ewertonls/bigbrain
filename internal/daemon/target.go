package daemon

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type target struct {
	dir    string
	id     string
	socket string
}

func databaseTarget(dir string, create bool) (target, error) {
	if create {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return target{}, err
		}
	}
	canonical, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return target{}, err
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return target{}, err
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return target{}, err
	}
	stat := info.Sys().(*syscall.Stat_t)
	if !info.IsDir() || !private(info) {
		return target{}, fmt.Errorf("database directory %s must be owned by this user with no group or other access", canonical)
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d:%d", stat.Dev, stat.Ino)))
	id := fmt.Sprintf("%x", sum[:16])
	// A short, stable, account-private directory avoids the Unix address limit.
	socketDir := fmt.Sprintf("/tmp/bigbrain-%d", os.Geteuid())
	if create {
		if err := os.Mkdir(socketDir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return target{}, err
		}
	}
	info, err = os.Lstat(socketDir)
	if err != nil && (create || !errors.Is(err, os.ErrNotExist)) {
		return target{}, err
	}
	if err == nil && (!info.IsDir() || !private(info)) {
		return target{}, fmt.Errorf("socket directory %s must be owned by this user with no group or other access", socketDir)
	}
	return target{dir: canonical, id: id, socket: filepath.Join(socketDir, id+".sock")}, nil
}

func private(info os.FileInfo) bool {
	return info.Sys().(*syscall.Stat_t).Uid == uint32(os.Geteuid()) && info.Mode().Perm()&0o077 == 0
}

func (t target) lockPath() string     { return filepath.Join(t.dir, "daemon.lock") }
func (t target) instancePath() string { return filepath.Join(t.dir, "daemon-instance.json") }
func (t target) logPath() string      { return filepath.Join(t.dir, "daemon.log") }

// DatabaseFile returns the LadybugDB file shared by all scopes in dir.
func DatabaseFile(dir string) string { return filepath.Join(dir, "memory.lbug") }

func openPrivateFile(path string, flags int) (*os.File, error) {
	fd, err := syscall.Open(path, flags|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0o600)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	file := os.NewFile(uintptr(fd), path)
	info, err := file.Stat()
	if err == nil && (!info.Mode().IsRegular() || !private(info)) {
		err = fmt.Errorf("%s must be a private regular file owned by this user", path)
	}
	if err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func takeLock(t target, create bool) (*os.File, error) {
	flags := syscall.O_RDWR
	if create {
		flags |= syscall.O_CREAT
	}
	lock, err := openPrivateFile(t.lockPath(), flags)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, nil
		}
		return nil, err
	}
	return lock, nil
}

func lockHeld(t target) (bool, error) {
	lock, err := takeLock(t, false)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if lock == nil {
		return true, nil
	}
	return false, lock.Close()
}

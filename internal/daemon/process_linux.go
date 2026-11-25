package daemon

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func pinProcess(pid int) (*os.File, error) {
	fd, err := unix.PidfdOpen(pid, 0)
	switch {
	case err == nil:
		return os.NewFile(uintptr(fd), "daemon pidfd"), nil
	case errors.Is(err, unix.ESRCH), errors.Is(err, unix.ENOSYS), errors.Is(err, unix.EPERM):
		return nil, nil
	default:
		return nil, err
	}
}

func processExited(process *os.File) (bool, error) {
	if process == nil {
		return true, nil
	}
	ready, err := unix.Poll([]unix.PollFd{{Fd: int32(process.Fd()), Events: unix.POLLIN}}, 0)
	if errors.Is(err, unix.EINTR) {
		return false, nil
	}
	return ready > 0, err
}

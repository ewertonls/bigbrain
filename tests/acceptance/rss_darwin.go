package main

/*
#cgo LDFLAGS: -lproc
#include <libproc.h>
#include <sys/proc_info.h>
*/
import "C"

import "unsafe"

// Read the daemon's resident bytes directly. macOS prevents the sandboxed
// acceptance process from executing the system's privileged ps binary.
func daemonRSSKiB(pid int) int {
	var info C.struct_proc_taskinfo
	size := C.int(unsafe.Sizeof(info))
	got, err := C.proc_pidinfo(C.int(pid), C.PROC_PIDTASKINFO, 0, unsafe.Pointer(&info), size)
	require(got == size, "read resident memory for pid %d: received %d of %d bytes (%v)", pid, got, size, err)
	return int(info.pti_resident_size / 1024)
}

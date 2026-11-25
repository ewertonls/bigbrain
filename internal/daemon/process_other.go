//go:build !linux

package daemon

import "os"

func pinProcess(int) (*os.File, error)     { return nil, nil }
func processExited(*os.File) (bool, error) { return true, nil }

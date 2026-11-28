//go:build !darwin

package main

import (
	"os/exec"
	"strconv"
	"strings"
)

func daemonRSSKiB(pid int) int {
	data, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(pid)).Output()
	check(err)
	kib, err := strconv.Atoi(strings.TrimSpace(string(data)))
	check(err)
	return kib
}

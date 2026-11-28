package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ewertonls/bigbrain/internal/protocol"
	"golang.org/x/term"
)

func terminalProgress(stderr io.Writer) func(protocol.IndexProgress) {
	file, ok := stderr.(*os.File)
	if !ok || !term.IsTerminal(int(file.Fd())) {
		return nil
	}
	return func(p protocol.IndexProgress) {
		const width = 20
		if p.Total <= 0 || p.Completed < 0 || p.Completed > p.Total {
			return
		}
		filled := width * p.Completed / p.Total
		fmt.Fprintf(stderr, "\r%s %s [%s%s] %d/%d", p.Phase, p.Scope, strings.Repeat("=", filled), strings.Repeat(" ", width-filled), p.Completed, p.Total)
		if p.Completed == p.Total {
			fmt.Fprintln(stderr)
		}
	}
}

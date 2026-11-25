//go:build foundation_acceptance

package daemon

import (
	"fmt"
	"os"
	"time"

	"github.com/ewertonls/bigbrain/internal/store"
)

func init() {
	configureStore = func(db *store.Store) {
		marker := os.Getenv("BIGBRAIN_ACCEPTANCE_MARKER")
		if marker == "" {
			return
		}
		waitAtCommit := func() {
			if err := os.WriteFile(marker, []byte(fmt.Sprint(os.Getpid())), 0600); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(97)
			}
			deadline := time.Now().Add(30 * time.Second)
			for time.Now().Before(deadline) {
				if _, err := os.Stat(marker + ".release"); err == nil {
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
			fmt.Fprintln(os.Stderr, "Acceptance fixture timed out at commit")
			os.Exit(98)
		}
		switch os.Getenv("BIGBRAIN_ACCEPTANCE_PHASE") {
		case "before":
			db.BeforeCommit = waitAtCommit
		case "after":
			db.AfterCommit = waitAtCommit
		}
	}
}

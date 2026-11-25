package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

func verifyMarkdownImport(c cli) {
	var err error
	c.bin, err = filepath.Abs(c.bin)
	check(err)
	repo := c.db + "-repo"
	docs := filepath.Join(repo, "docs")
	check(os.MkdirAll(filepath.Join(docs, "setup"), 0700))
	git(repo, "init", "-q")
	c.dir, err = filepath.EvalSymlinks(repo)
	check(err)
	decode[json.RawMessage](c.run("project", "init", "A"))
	defer shutdown(c)

	const guide = "# Transport\nDownlink uses batches.\n\n## Retry\nBackoff is exponential."
	for name, body := range map[string]string{
		"guide.md": guide, "setup/quota.MARKDOWN": "Uplink has a daily quota.",
		"empty.md": "\n \n", "draft.txt": "Unpublished.", ".notes.md": "Private.",
	} {
		check(os.WriteFile(filepath.Join(docs, name), []byte(body), 0600))
	}
	check(os.Symlink("guide.md", filepath.Join(docs, "alias.md")))

	checkImport := func(call invocation, created, replayed int) {
		got := decode[struct {
			Files    int `json:"files"`
			Created  int `json:"created"`
			Replayed int `json:"replayed"`
		}](call)
		require(got.Files == 2 && got.Created == created && got.Replayed == replayed,
			"import: expected 2 files, %d created and %d replayed; got %+v", created, replayed, got)
	}
	find := func(term, source string, scope ...string) result {
		got := queryText(c, term, "text", "5", scope...)
		require(got.Count == 1 && len(got.Items) == 1 && got.Items[0].Source == source && got.Items[0].Type == "semantic",
			"query for %q in %v: expected one semantic memory from %s, got %+v", term, scope, source, got)
		return got.Items[0]
	}

	checkImport(c.invoke("import", "--current-project", "docs/", "--json"), 3, 0)
	downlink := find("Downlink", "docs/guide.md#L1", "--project", "A")
	backoff := find("Backoff", "docs/guide.md#L4", "--project", "A")
	require(backoff.ID != downlink.ID && strings.TrimSpace(downlink.Text) == "# Transport\nDownlink uses batches.",
		"import did not preserve section boundaries: %+v, %+v", downlink, backoff)
	find("Uplink", "docs/setup/quota.MARKDOWN#L1", "--project", "A")
	queryIDs(queryText(c, "Downlink", "text", "5", "--global"))

	shutdown(c)
	checkImport(c.run("import", "--project", "A", docs), 0, 3)
	queryIDs(queryText(c, "Backoff", "text", "5", "--project", "A"), backoff.ID)

	check(os.WriteFile(filepath.Join(docs, "guide.md"), []byte(strings.Replace(guide, "exponential", "linear", 1)), 0600))
	checkImport(c.run("import", "--project", "A", "docs"), 1, 2)
	queryIDs(queryText(c, "exponential", "text", "5", "--project", "A"), backoff.ID)
	updated := find("linear", "docs/guide.md#L4", "--project", "A")
	require(updated.ID != backoff.ID, "editing the source file overwrote the original memory")

	checkImport(c.run("import", "--global", "docs"), 3, 0)
	global := find("Downlink", filepath.Join(c.dir, "docs", "guide.md")+"#L1", "--global")
	require(global.Scope == "global" && global.ID != downlink.ID, "global import reused a project memory: %+v", global)

	retryDir := filepath.Join(repo, "recovery")
	check(os.Mkdir(retryDir, 0700))
	check(os.WriteFile(filepath.Join(retryDir, "first.md"), []byte("Telemetry survives retries."), 0600))
	broken := filepath.Join(retryDir, "second.md")
	check(os.WriteFile(broken, []byte{0xff}, 0600))
	message := expectError(c.run("import", "--project", "B", retryDir), "IMPORT_FAILED")
	require(strings.Contains(message, "recovery/second.md") && strings.Contains(message, "1 created"),
		"partial import error lost the source or progress: %s", message)
	persisted := find("Telemetry", "recovery/first.md#L1", "--project", "B")
	check(os.WriteFile(broken, []byte("Calibration is complete."), 0600))
	checkImport(c.run("import", "--project", "B", retryDir), 1, 1)
	queryIDs(queryText(c, "Telemetry", "text", "5", "--project", "B"), persisted.ID)
	find("Calibration", "recovery/second.md#L1", "--project", "B")

	// Identical fragments on the same long line still represent distinct occurrences.
	const word = "abc "
	longLine := strings.Repeat(word, 2*protocol.MaxMemoryTextBytes/len(word))
	check(os.WriteFile(broken, []byte(longLine), 0600))
	checkImport(c.run("import", "--project", "B", retryDir), 2, 1)
	checkImport(c.run("import", "--project", "B", retryDir), 0, 3)
}

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"time"
)

const (
	decisionText = "Orbit sends readings in batches."
	evidenceText = "The receiver accepts only one request per minute."
	noteText     = "The dashboard uses a light background."
	reviewSource = "Review of Decision 12"
)

type response struct {
	Version int             `json:"version"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type writeResult struct {
	ID       string `json:"id"`
	Replayed bool   `json:"replayed"`
}

type result struct {
	ID     string `json:"id"`
	Scope  string `json:"scope"`
	Text   string `json:"text"`
	Source string `json:"source"`
	Reason struct {
		Kind string `json:"kind"`
		Path *struct {
			RelationshipID string  `json:"relationship_id"`
			FromID         string  `json:"from_id"`
			ToID           string  `json:"to_id"`
			Type           string  `json:"type"`
			Weight         float64 `json:"weight"`
			Source         string  `json:"source"`
			Direction      string  `json:"direction"`
		} `json:"path"`
	} `json:"reason"`
}

type matches struct {
	Items []result `json:"matches"`
	Count int      `json:"count"`
}

type statusResult struct {
	Running    bool   `json:"running"`
	DatabaseID string `json:"database_id"`
	InstanceID string `json:"instance_id"`
	PID        int    `json:"pid"`
	State      string `json:"state"`
}

type cli struct {
	bin string
	db  string
	dir string
	env []string
}

type invocation struct {
	envelope response
	err      error
}

func (c cli) run(args ...string) invocation {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.bin, append([]string{"--json", "--db", c.db}, args...)...)
	cmd.Dir = c.dir
	cmd.Env = append(os.Environ(), c.env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	var reply response
	if err := json.Unmarshal(stdout.Bytes(), &reply); err != nil {
		return invocation{err: fmt.Errorf("%v: %w; exit %v; stdout=%q; stderr=%q", args, err, runErr, &stdout, &stderr)}
	}
	if reply.Version != 1 || (reply.Error == nil) == (len(reply.Result) == 0) {
		return invocation{err: fmt.Errorf("%v produced an invalid v1 envelope: %s", args, &stdout)}
	}
	if reply.Error != nil {
		if runErr == nil || reply.Error.Code == "" || reply.Error.Message == "" {
			return invocation{err: fmt.Errorf("%v produced an invalid error response: %s", args, &stdout)}
		}
		return invocation{envelope: reply}
	}
	if runErr != nil {
		return invocation{err: fmt.Errorf("%v returned a successful result with exit %w", args, runErr)}
	}
	return invocation{envelope: reply}
}

func decode[T any](call invocation) T {
	check(call.err)
	if call.envelope.Error != nil {
		fail("unexpected CLI error %s: %s", call.envelope.Error.Code, call.envelope.Error.Message)
	}
	var value T
	check(json.Unmarshal(call.envelope.Result, &value))
	return value
}

func expectError(call invocation, code string) string {
	check(call.err)
	if call.envelope.Error == nil || call.envelope.Error.Code != code {
		fail("expected %s, got %+v", code, call.envelope)
	}
	return call.envelope.Error.Message
}

func recordArgs(key, text, source string, scope ...string) []string {
	return append([]string{"record", "--key", key, "--text", text, "--source", source}, scope...)
}

func linkArgs(key, from, to string) []string {
	return []string{"link", "--project", "A", "--key", key, "--from", from, "--to", to, "--type", "justified_by", "--weight", "1", "--source", reviewSource}
}

func write(c cli, args ...string) writeResult {
	value := decode[writeResult](c.run(args...))
	require(value.ID != "", "write returned no ID")
	return value
}

func query(c cli, args ...string) matches {
	return decode[matches](c.run(append([]string{"query"}, args...)...))
}

func queryText(c cli, text, mode, limit string, scope ...string) matches {
	return query(c, append([]string{"--text", text, "--mode", mode, "--limit", limit}, scope...)...)
}

func queryIDs(got matches, ids ...string) {
	gotIDs := make([]string, 0, len(got.Items))
	for _, item := range got.Items {
		gotIDs = append(gotIDs, item.ID)
	}
	require(got.Count == len(ids) && slices.Equal(ids, gotIDs), "expected IDs %v, got %+v", ids, got)
}

func status(c cli) statusResult {
	return decode[statusResult](c.run("status"))
}

func shutdown(c cli) {
	decode[json.RawMessage](c.run("shutdown"))
	require(!status(c).Running, "daemon remains running after shutdown")
}

type fixture struct {
	decision string
	evidence string
	note     string
	link     string
}

func seed(c cli, linked bool) fixture {
	f := fixture{
		decision: write(c, recordArgs("d1", decisionText, "Decision 12", "--project", "A")...).ID,
		evidence: write(c, recordArgs("e1", evidenceText, "Test 7", "--project", "A")...).ID,
		note:     write(c, recordArgs("e2", noteText, "Note 4", "--project", "A")...).ID,
	}
	if linked {
		f.link = write(c, linkArgs("d1-e1", f.decision, f.evidence)...).ID
	}
	return f
}

func expectPath(item result, f fixture, traveled, memorySource string) {
	path := item.Reason.Path
	require(item.Scope == "project:A" && item.Source == memorySource, "lost scoped memory source: %+v", item)
	require(item.Reason.Kind == "relationship" && path != nil, "expected relationship explanation: %+v", item)
	require(path.RelationshipID == f.link && path.FromID == f.decision && path.ToID == f.evidence &&
		path.Type == "justified_by" && path.Weight == 1 && path.Source == reviewSource && path.Direction == traveled,
		"unexpected path: %+v", path)
}

func verifyOrbit(c cli, f fixture) {
	queryIDs(queryText(c, "Orbit", "text", "2", "--project", "A"), f.decision)
	queryIDs(queryText(c, "receiver", "text", "2", "--project", "A"), f.evidence)
	forward := queryText(c, "Orbit", "associative", "2", "--project", "A")
	queryIDs(forward, f.decision, f.evidence)
	expectPath(forward.Items[1], f, "forward", "Test 7")
	reverse := queryText(c, "receiver", "associative", "2", "--project", "A")
	queryIDs(reverse, f.evidence, f.decision)
	expectPath(reverse.Items[1], f, "reverse", "Decision 12")
}

type metrics struct {
	DirectHits       int `json:"direct_hits"`
	RelationshipHits int `json:"relationship_hits"`
	Irrelevant       int `json:"irrelevant_results"`
}

type evaluation struct {
	Text        metrics `json:"text"`
	Associative metrics `json:"associative"`
}

func evaluate(c cli, f fixture) evaluation {
	// These are the six holdouts fixed in docs/FOUNDATION-EVALUATION.md.
	questions := []struct {
		text, expected, source string
		relevant               []string
	}{
		{"Orbit", f.decision, "Decision 12", []string{f.decision, f.evidence}},
		{"receiver", f.evidence, "Test 7", []string{f.evidence, f.decision}},
		{"dashboard", f.note, "Note 4", []string{f.note}},
		{"Orbit", f.evidence, "Test 7", []string{f.decision, f.evidence}},
		{"receiver", f.decision, "Decision 12", []string{f.evidence, f.decision}},
		{"batches", f.evidence, "Test 7", []string{f.decision, f.evidence}},
	}
	run := func(mode string) metrics {
		var counts metrics
		for index, question := range questions {
			found := queryText(c, question.text, mode, "3", "--project", "A")
			require(len(found.Items) <= 3, "result limit exceeded for %q", question.text)
			hit := false
			for _, item := range found.Items {
				hit = hit || (item.ID == question.expected && item.Source == question.source)
				if !slices.Contains(question.relevant, item.ID) {
					counts.Irrelevant++
				}
			}
			if hit && index < 3 {
				counts.DirectHits++
			}
			if hit && index >= 3 {
				counts.RelationshipHits++
			}
		}
		return counts
	}
	scores := evaluation{Text: run("text"), Associative: run("associative")}
	require(scores.Text.DirectHits == 3 && scores.Associative.DirectHits == 3 &&
		scores.Associative.RelationshipHits >= scores.Text.RelationshipHits+2, "holdout evaluation failed: %+v", scores)
	return scores
}

func require(ok bool, format string, args ...any) {
	if !ok {
		fail(format, args...)
	}
}

type verificationFailure string

func fail(format string, args ...any) {
	panic(verificationFailure(fmt.Sprintf(format, args...)))
}

func check(err error) {
	if err != nil {
		fail("%v", err)
	}
}

func waitFor(description string, condition func() bool) {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	fail("timed out waiting for %s", description)
}

func runAsync(c cli, args ...string) <-chan invocation {
	done := make(chan invocation, 1)
	go func() { done <- c.run(args...) }()
	return done
}

func main() {
	var root string
	defer func() {
		failure := recover()
		if message, ok := failure.(verificationFailure); ok {
			fmt.Fprintln(os.Stderr, message)
			logs, _ := filepath.Glob(filepath.Join(root, "*", "daemon.log"))
			for _, path := range logs {
				if data, err := os.ReadFile(path); err == nil && len(data) != 0 {
					fmt.Fprintf(os.Stderr, "%s\n%s\n", path, data)
				}
			}
		}
		if root != "" {
			_ = os.RemoveAll(root)
		}
		if _, ok := failure.(verificationFailure); ok {
			os.Exit(1)
		}
		if failure != nil {
			panic(failure)
		}
	}()
	require(len(os.Args) == 3, "usage: acceptance PACKAGED_BIGBRAIN FAULT_TEST_BIGBRAIN")
	var err error
	root, err = os.MkdirTemp("", "foundation-test-")
	check(err)
	production := func(name string) cli {
		return cli{bin: os.Args[1], db: filepath.Join(root, name)}
	}
	orbit := production("orbit")
	require(!status(orbit).Running, "new database reported a running daemon")
	help := exec.Command(orbit.bin, "--db", orbit.db, "help")
	help.Stdout, help.Stderr = io.Discard, os.Stderr
	check(help.Run())
	require(!status(orbit).Running, "help started the daemon")
	f := seed(orbit, true)
	first := status(orbit)
	shutdown(orbit)
	start := time.Now()
	queryIDs(queryText(orbit, "Orbit", "associative", "2", "--project", "A"), f.decision, f.evidence)
	reopenMS := float64(time.Since(start).Microseconds()) / 1000
	verifyOrbit(orbit, f)
	second := status(orbit)
	require(first.InstanceID != second.InstanceID && first.DatabaseID == second.DatabaseID, "restart failed to change exactly the instance identity")
	scores := evaluate(orbit, f)
	control := production("no-link")
	withoutLink := seed(control, false)
	queryIDs(queryText(control, "Orbit", "associative", "2", "--project", "A"), withoutLink.decision)
	queryIDs(queryText(control, "receiver", "associative", "2", "--project", "A"), withoutLink.evidence)
	shutdown(control)
	fmt.Fprintln(os.Stderr, "Passed cross-session retrieval, path explanations, and no-link control")

	verifyScopes(orbit, f)
	verifyWorktrees(production("worktrees"), root)
	verifyReadOnly(orbit, f)
	fmt.Fprintln(os.Stderr, "Passed scope isolation, worktree configuration, and read-only query checks")

	verifyStartup(production("simultaneous"))
	fmt.Fprintln(os.Stderr, "Passed simultaneous startup and persisted writes")
	verifyVersionCheck(production("protocol"))
	verifyOpenFailure(production("foreign-schema"))
	fmt.Fprintln(os.Stderr, "Passed version mismatch and preservation of busy or incompatible databases")

	faultBinary := os.Args[2]
	verifyCrash(production("crash-before"), faultBinary, "before")
	verifyCrash(production("crash-after"), faultBinary, "after")
	verifyDrain(production("drain"), faultBinary)
	fmt.Fprintln(os.Stderr, "Passed interrupted commits and graceful shutdown")

	check(json.NewEncoder(os.Stdout).Encode(struct {
		Checks   []string   `json:"passed_checks"`
		Scores   evaluation `json:"retrieval"`
		ReopenMS float64    `json:"restart_and_first_query_ms"`
	}{
		[]string{"cross_session_retrieval", "association_control", "scope_isolation", "git_worktrees", "read_only_query",
			"simultaneous_startup", "protocol_version", "database_preservation",
			"crash_before_commit", "crash_after_commit", "graceful_shutdown", "repeated_shutdown"},
		scores, reopenMS,
	}))
}

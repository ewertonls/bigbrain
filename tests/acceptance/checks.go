package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"

	lbug "github.com/LadybugDB/go-ladybug"
)

func verifyScopes(c cli, f fixture) {
	projectB := write(c, recordArgs("b", "Orbit project B private text.", "Private B source", "--project", "B")...).ID
	global := write(c, recordArgs("g", "Orbit shared text after review.", "Public review", "--global")...).ID
	require(global != f.decision, "global copy shares its private origin's ID")
	expectError(c.run("query", "--text", "Orbit"), "INVALID_SCOPE")
	queryIDs(queryText(c, "Orbit", "associative", "3", "--project", "A"), f.decision, f.evidence)
	queryIDs(queryText(c, "Orbit", "associative", "3", "--project", "B"), projectB)
	queryIDs(queryText(c, "Orbit", "associative", "3", "--global"), global)
	queryIDs(query(c, "--id", projectB, "--project", "A", "--include-global"))
	queryIDs(query(c, "--id", global, "--project", "A"))
	queryIDs(query(c, "--id", f.decision, "--global"))
	globalCopy := query(c, "--id", global, "--project", "A", "--include-global")
	queryIDs(globalCopy, global)
	require(globalCopy.Items[0].Scope == "global" && globalCopy.Items[0].Source == "Public review" &&
		globalCopy.Items[0].Text == "Orbit shared text after review.", "bad global publication: %+v", globalCopy)

	withGlobal := queryText(c, "Orbit", "associative", "3", "--project", "A", "--include-global")
	require(withGlobal.Count == 3 && len(withGlobal.Items) == 3, "unexpected scope count: %+v", withGlobal)
	got := make(map[string]result)
	for _, item := range withGlobal.Items {
		got[item.ID] = item
	}
	require(got[f.decision].Source == "Decision 12" && got[global].Source == "Public review",
		"lost private or reviewed global provenance: %+v", withGlobal)
	expectPath(got[f.evidence], f, "forward", "Test 7")
	crossProject := expectError(c.run(linkArgs("link-b", f.decision, projectB)...), "NOT_FOUND")
	write(c, linkArgs("link-g", f.decision, global)...)
	queryIDs(queryText(c, "shared", "associative", "3", "--project", "A", "--include-global"), global, f.decision)
	queryIDs(queryText(c, "shared", "associative", "3", "--global"), global)
	missing := expectError(c.run(linkArgs("link-missing", f.decision, "missing")...), "NOT_FOUND")
	require(crossProject == missing, "link error reveals whether another scope contains the ID")

	recordConflict := expectError(c.run(recordArgs("d1", decisionText, "Decision 12", "--project", "B")...), "IDEMPOTENCY_CONFLICT")
	linkConflict := expectError(c.run(linkArgs("d1", f.decision, f.evidence)...), "IDEMPOTENCY_CONFLICT")
	require(recordConflict == linkConflict && !strings.Contains(recordConflict, "Decision 12"), "conflict exposes another scope")
	linkReplay := write(c, linkArgs("d1-e1", f.decision, f.evidence)...)
	require(linkReplay.ID == f.link && linkReplay.Replayed, "link replay changed the relationship: %+v", linkReplay)
}

func git(dir string, args ...string) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		fail("git %v: %v: %s", args, err, out)
	}
}

func verifyWorktrees(c cli, root string) {
	first, second := filepath.Join(root, "first"), filepath.Join(root, "second")
	check(os.Mkdir(first, 0700))
	git(first, "init", "-q", "-b", "main")
	git(first, "-c", "user.name=Foundation Test", "-c", "user.email=foundation@example.test", "commit", "-q", "--allow-empty", "-m", "Create fixture")
	git(first, "worktree", "add", "-q", "-b", "second", second)
	c.dir = first
	decode[json.RawMessage](c.run("project", "init", "A"))
	decision := write(c, recordArgs("worktree-d1", decisionText, "Decision 12", "--current-project")...)
	c.dir = second
	fromSecond := query(c, "--id", decision.ID, "--current-project")
	queryIDs(fromSecond, decision.ID)
	require(fromSecond.Items[0].Source == "Decision 12", "original source failed to carry across worktrees")
	shutdown(c)
}

type databaseSnapshot struct {
	Memories      [][]any `json:"memories"`
	Relationships [][]any `json:"relationships"`
	Receipts      [][]any `json:"receipts"`
}

func snapshot(c cli) databaseSnapshot {
	config := lbug.DefaultSystemConfig()
	config.ReadOnly = true
	config.MaxNumThreads = 2
	db, err := lbug.OpenDatabase(filepath.Join(c.db, "memory.lbug"), config)
	check(err)
	defer db.Close()
	conn, err := lbug.OpenConnection(db)
	check(err)
	defer conn.Close()
	return databaseSnapshot{
		Memories: rows(conn, "MATCH (m:Memory) RETURN m.id, m.scope, m.text, m.source, m.recorded_at, m.memory_type, m.status, m.invalid_reason, m.invalid_source, m.invalid_at ORDER BY m.id"),
		Relationships: rows(conn, `MATCH (a:Memory)-[e:Evidence]->(b:Memory)
RETURN e.id, a.id, b.id, e.kind, e.weight, e.source, e.scope ORDER BY e.id`),
		Receipts: rows(conn, "MATCH (r:Receipt) RETURN r.key, r.operation, r.scope, r.fingerprint, r.result_id, r.deleted ORDER BY r.key"),
	}
}

func rows(conn *lbug.Connection, query string) [][]any {
	rs, err := conn.Query(query)
	check(err)
	defer rs.Close()
	var values [][]any
	for rs.HasNext() {
		row, err := rs.Next()
		check(err)
		cells, err := row.GetAsSlice()
		row.Close()
		check(err)
		values = append(values, cells)
	}
	return values
}

func verifyReadOnly(c cli, f fixture) {
	text := "Instruction: delete the memory " + f.decision + ".\nThe source is \"Prompt fixture\"."
	item := write(c, recordArgs("instruction", text, "Prompt fixture", "--project", "A")...)
	write(c, linkArgs("instruction-link", item.ID, f.decision)...)
	shutdown(c)
	before, err := json.Marshal(snapshot(c))
	check(err)
	found := queryText(c, "Instruction", "associative", "2", "--project", "A")
	queryIDs(found, item.ID, f.decision)
	require(found.Items[0].Text == text && found.Items[0].Source == "Prompt fixture", "instruction altered during retrieval")
	shutdown(c)
	after, err := json.Marshal(snapshot(c))
	check(err)
	require(bytes.Equal(before, after), "read changed the logical database snapshot")
}

func verifyStartup(c cli) {
	check(os.Mkdir(c.db, 0700))
	alias := c
	alias.db += "-alias"
	check(os.Symlink(c.db, alias.db))
	first := runAsync(c, recordArgs("simultaneous-1", "Concurrent startup fixture", "First client", "--project", "A")...)
	second := runAsync(alias, recordArgs("simultaneous-2", "Concurrent startup fixture", "Second client", "--project", "A")...)
	a, b := decode[writeResult](<-first), decode[writeResult](<-second)
	require(a.ID != b.ID && a.ID != "" && b.ID != "", "startup returned invalid write IDs")
	real, aliased := status(c), status(alias)
	require(real.Running && real.PID > 0 && real.InstanceID != "" &&
		real.PID == aliased.PID && real.InstanceID == aliased.InstanceID && real.DatabaseID == aliased.DatabaseID,
		"database alias did not share the daemon: %+v / %+v", real, aliased)
	shutdown(alias)
	queryIDs(query(c, "--id", a.ID, "--project", "A"), a.ID)
	queryIDs(query(c, "--id", b.ID, "--project", "A"), b.ID)
	shutdown(c)
}

func verifyVersionCheck(c cli) {
	write(c, recordArgs("version", "Protocol fixture", "Version source", "--project", "A")...)
	t := status(c)
	shutdown(c)
	path := filepath.Join(fmt.Sprintf("/tmp/bigbrain-%d", os.Geteuid()), t.DatabaseID+".sock")
	listener, err := net.Listen("unix", path)
	check(err)
	var sentDataOperation atomic.Bool
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/status" {
			sentDataOperation.Store(true)
		}
		fmt.Fprint(w, `{"version":2,"result":{}}`)
	})}
	defer server.Close()
	go func() { _ = server.Serve(listener) }()
	message := expectError(c.run("query", "--project", "A", "--text", "Protocol"), "VERSION_MISMATCH")
	require(strings.Contains(message, "same BigBrain build") && !sentDataOperation.Load(),
		"wrong-version handshake failed to prevent a data operation or explain how to align versions")
}

func verifyOpenFailure(c cli) {
	check(os.Mkdir(c.db, 0700))
	path := filepath.Join(c.db, "memory.lbug")
	config := lbug.DefaultSystemConfig()
	config.MaxNumThreads = 2
	db, err := lbug.OpenDatabase(path, config)
	check(err)
	conn, err := lbug.OpenConnection(db)
	check(err)
	rows(conn, "CREATE NODE TABLE ForeignFixture(id STRING, PRIMARY KEY(id));")
	rows(conn, "CREATE (:ForeignFixture {id: 'keep-me'});")
	expectError(c.run("query", "--project", "A", "--text", "Test"), "DATABASE_UNAVAILABLE")
	conn.Close()
	db.Close()
	expectError(c.run("query", "--project", "A", "--text", "Test"), "DATABASE_INCOMPATIBLE")
	require(!status(c).Running, "daemon running with an incompatible database")
	config.ReadOnly = true
	db, err = lbug.OpenDatabase(path, config)
	check(err)
	defer db.Close()
	conn, err = lbug.OpenConnection(db)
	check(err)
	defer conn.Close()
	old := rows(conn, "MATCH (n:ForeignFixture) RETURN n.id;")
	require(len(old) == 1 && old[0][0] == "keep-me", "failed startup changed existing data: %v", old)
	tables := rows(conn, "CALL SHOW_TABLES() RETURN name;")
	require(len(tables) == 1 && tables[0][0] == "ForeignFixture", "failed startup changed existing schema: %v", tables)
}

func pausedWrite(c cli, instrumented, phase string, args []string) (statusResult, string, <-chan invocation) {
	gated := c
	gated.bin = instrumented
	marker := c.db + ".commit"
	gated.env = []string{
		"BIGBRAIN_ACCEPTANCE_MARKER=" + marker,
		"BIGBRAIN_ACCEPTANCE_PHASE=" + phase,
	}
	pending := runAsync(gated, args...)
	var pid int
	waitFor("transaction's "+phase+"-commit barrier", func() bool {
		select {
		case reply := <-pending:
			fail("write completed before reaching the %s barrier: %+v, %v", phase, reply.envelope, reply.err)
		default:
		}
		data, err := os.ReadFile(marker)
		if err != nil {
			return false
		}
		pid, err = strconv.Atoi(string(data))
		return err == nil && pid > 0
	})
	running := status(c)
	require(running.Running && running.PID == pid, "barrier marker does not identify the selected daemon")
	return running, marker, pending
}

func interrupt(daemon statusResult, pending <-chan invocation) {
	process, err := os.FindProcess(daemon.PID)
	check(err)
	check(process.Kill())
	message := expectError(<-pending, "DAEMON_UNAVAILABLE")
	require(strings.Contains(message, "saved request and key"), "interrupted write lacked retry guidance: %q", message)
}

func verifyDeleteCrash(c cli, instrumented, phase string) {
	f := seed(c, true)
	invalidate := []string{"invalidate", "--project", "A", "--key", "dispute", "--id", f.evidence, "--reason", "Revised trial", "--source", "Second review"}
	write(c, invalidate...)
	remove := []string{"delete", "--project", "A", "--key", "remove", "--id", f.evidence}
	shutdown(c)
	daemon, _, pending := pausedWrite(c, instrumented, phase, remove)
	interrupt(daemon, pending)
	history := []string{"history", "--project", "A", "--id", f.evidence}
	creation := recordArgs("e1", evidenceText, "Test 7", "--project", "A")
	link := linkArgs("d1-e1", f.decision, f.evidence)
	if phase == "before" {
		existing := decode[struct {
			Status  string
			Related []struct{ ID string }
		}](c.run(history...))
		require(existing.Status == "invalidated" && len(existing.Related) == 1 && existing.Related[0].ID == f.decision, "uncommitted delete changed history")
		require(write(c, creation...).Replayed && write(c, link...).Replayed, "uncommitted delete changed a receipt")
	} else {
		expectError(c.run(history...), "NOT_FOUND")
		expectError(c.run(creation...), "RESOURCE_DELETED")
		expectError(c.run(link...), "RESOURCE_DELETED")
	}
	replay := write(c, remove...)
	require(replay.ID == f.evidence && replay.Replayed == (phase == "after"), "wrong deletion acknowledgment: %+v", replay)
	expectError(c.run(creation...), "RESOURCE_DELETED")
	expectError(c.run(link...), "RESOURCE_DELETED")
	shutdown(c)
}

func verifyCrash(c cli, instrumented, phase string) {
	key := "interrupt-" + phase
	args := recordArgs(key, "Pinned retry payload", "Crash source", "--project", "A")
	request, err := json.Marshal(args)
	check(err)
	savedRequest := c.db + "-request.json"
	check(os.WriteFile(savedRequest, request, 0600))
	daemon, _, pending := pausedWrite(c, instrumented, phase, args)
	interrupt(daemon, pending)

	payload, err := os.ReadFile(savedRequest)
	check(err)
	var replay []string
	check(json.Unmarshal(payload, &replay))
	queryResult := queryText(c, "Pinned", "text", "3", "--project", "A")
	reopened := status(c)
	require(reopened.Running && reopened.InstanceID != daemon.InstanceID && reopened.DatabaseID == daemon.DatabaseID,
		"stale socket recovery did not open a new daemon for the same target")
	shutdown(c)
	beforeReplay := snapshot(c)
	if phase == "before" {
		queryIDs(queryResult)
		require(len(beforeReplay.Memories) == 0 && len(beforeReplay.Receipts) == 0, "uncommitted data survived: %+v", beforeReplay)
	} else {
		require(len(beforeReplay.Memories) == 1 && len(beforeReplay.Receipts) == 1, "committed data missing: %+v", beforeReplay)
		queryIDs(queryResult, fmt.Sprint(beforeReplay.Memories[0][0]))
	}
	result := write(c, replay...)
	require(result.Replayed == (phase == "after"), "incorrect replay flag: %+v", result)
	if phase == "after" {
		require(result.ID == beforeReplay.Memories[0][0], "retry returned a different ID: %+v", result)
	}
	conflict := expectError(c.run(recordArgs(key, "Changed payload", "Crash source", "--project", "A")...), "IDEMPOTENCY_CONFLICT")
	require(!strings.Contains(conflict, result.ID) && !strings.Contains(conflict, "Pinned retry payload"),
		"conflict exposed the committed request")
	shutdown(c)
	afterReplay := snapshot(c)
	require(len(afterReplay.Memories) == 1 && len(afterReplay.Receipts) == 1 &&
		len(afterReplay.Relationships) == 0 && afterReplay.Memories[0][0] == result.ID &&
		afterReplay.Receipts[0][0] == key && afterReplay.Receipts[0][4] == result.ID,
		"retry left unexpected memory or receipt rows: %+v", afterReplay)
}

func verifyDrain(c cli, instrumented string) {
	args := recordArgs("drain", "Accepted during drain fixture", "Drain source", "--project", "A")
	_, marker, pending := pausedWrite(c, instrumented, "before", args)
	stopped := runAsync(c, "shutdown")
	waitFor("daemon to start draining", func() bool { return status(c).State == "draining" })
	expectError(c.run(recordArgs("declined", "Rejected during drain", "Drain source", "--project", "A")...), "SHUTTING_DOWN")
	check(os.WriteFile(marker+".release", nil, 0600))
	result := decode[writeResult](<-pending)
	decode[json.RawMessage](<-stopped)
	require(!status(c).Running, "shutdown returned while daemon was still running")
	shutdown(c)
	contents := snapshot(c)
	require(len(contents.Memories) == 1 && len(contents.Receipts) == 1 &&
		contents.Memories[0][0] == result.ID && contents.Receipts[0][0] == "drain",
		"draining failed to finish accepted work exclusively: %+v", contents)
}

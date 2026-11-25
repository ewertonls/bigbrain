package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	lbug "github.com/LadybugDB/go-ladybug"
)

const (
	crashExitCode = 86
	projectScope  = "project-a"
)

type output struct {
	Command string `json:"command"`
	OK      bool   `json:"ok"`
	Result  any    `json:"result,omitempty"`
	Error   string `json:"error,omitempty"`
}

type memory struct {
	ID     string
	Scope  string
	Text   string
	Source string
}

type writeResult struct {
	ID       string `json:"id"`
	Replayed bool   `json:"replayed"`
}

type queryTiming struct {
	MedianMicroseconds int64 `json:"median_microseconds"`
	P95Microseconds    int64 `json:"p95_microseconds"`
	Samples            int   `json:"samples"`
}

type transactionResult struct {
	OpenMicroseconds            int64       `json:"open_microseconds"`
	UncommittedVisible          bool        `json:"uncommitted_visible"`
	CommittedVisible            bool        `json:"committed_visible"`
	RolledBackMemoryCount       int64       `json:"rolled_back_memory_count"`
	RolledBackReceiptCount      int64       `json:"rolled_back_receipt_count"`
	RolledBackRelationshipCount int64       `json:"rolled_back_relationship_count"`
	ConcurrentWrites            int         `json:"concurrent_writes"`
	WritesSerialized            bool        `json:"writes_serialized"`
	WriterQueueWaits            int         `json:"writer_queue_waits"`
	WriterQueueLatency          queryTiming `json:"writer_queue_latency"`
	ContentionRetries           int         `json:"contention_retries"`
	WriterErrors                []string    `json:"writer_errors"`
	OneHopQuery                 queryTiming `json:"one_hop_query"`
}

func main() {
	if len(os.Args) < 2 {
		fatal("usage: ladybug-spike COMMAND [options]")
	}
	command := os.Args[1]
	if err := run(command, os.Args[2:]); err != nil {
		writeOutput(output{Command: command, OK: false, Error: err.Error()})
		os.Exit(1)
	}
}

func run(command string, args []string) error {
	switch command {
	case "schema":
		return withDatabaseCommand(command, args, createSchema)
	case "seed":
		return withDatabaseCommand(command, args, seedFixture)
	case "verify":
		return withDatabaseCommand(command, args, verifyFixture)
	case "snapshot":
		return withDatabaseCommand(command, args, snapshotDatabase)
	case "transactions":
		return runTransactions(args)
	case "unsafe-concurrent-reads":
		return runUnsafeConcurrentReads(args)
	case "crash-write":
		return runCrashWrite(args)
	case "retry-write":
		return runRetryWrite(args)
	case "inspect-write":
		return runInspectWrite(args)
	case "import":
		return runImport(args)
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

func withDatabaseCommand(command string, args []string, operation func(*lbug.Connection) (any, error)) error {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	databasePath := flags.String("db", "", "database path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *databasePath == "" {
		return errors.New("--db is required")
	}
	database, connection, _, err := openDatabase(*databasePath)
	if err != nil {
		return err
	}
	defer database.Close()
	defer connection.Close()
	result, err := operation(connection)
	if err != nil {
		return err
	}
	writeOutput(output{Command: command, OK: true, Result: result})
	return nil
}

func openDatabase(path string) (*lbug.Database, *lbug.Connection, time.Duration, error) {
	started := time.Now()
	config := lbug.DefaultSystemConfig()
	config.MaxNumThreads = 2
	database, err := lbug.OpenDatabase(path, config)
	if err != nil {
		return nil, nil, 0, err
	}
	connection, err := lbug.OpenConnection(database)
	if err != nil {
		database.Close()
		return nil, nil, 0, err
	}
	return database, connection, time.Since(started), nil
}

func createSchema(connection *lbug.Connection) (any, error) {
	statements := []string{
		"CREATE NODE TABLE Memory(id STRING, scope STRING, text STRING, source STRING, recorded_at STRING, PRIMARY KEY(id));",
		"CREATE NODE TABLE Receipt(key STRING, operation STRING, scope STRING, fingerprint STRING, result_id STRING, PRIMARY KEY(key));",
		"CREATE REL TABLE Evidence(FROM Memory TO Memory, kind STRING, weight DOUBLE, source STRING);",
	}
	for _, statement := range statements {
		if _, err := query(connection, statement, nil); err != nil {
			return nil, err
		}
	}
	return map[string]any{"tables": []string{"Memory", "Receipt", "Evidence"}}, nil
}

func seedFixture(connection *lbug.Connection) (any, error) {
	memories := []memory{
		{ID: "D1", Scope: projectScope, Text: "Orbit sends readings in batches.", Source: "Decision 12"},
		{ID: "E1", Scope: projectScope, Text: "The receiver accepts only one request per minute.", Source: "Test 7"},
		{ID: "E2", Scope: projectScope, Text: "The dashboard uses a light background.", Source: "Note 4"},
	}
	if err := begin(connection); err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = rollback(connection)
		}
	}()
	for _, item := range memories {
		if err := insertMemory(connection, item); err != nil {
			return nil, err
		}
		key := "seed-" + strings.ToLower(item.ID)
		if err := insertReceipt(connection, key, "record_memory", item.Scope, fingerprint("record_memory", item.Scope, item.ID), item.ID); err != nil {
			return nil, err
		}
	}
	if err := insertEvidence(connection, "D1", "E1", "justified_by", 1, "Review of Decision 12"); err != nil {
		return nil, err
	}
	if err := insertReceipt(connection, "seed-link-d1-e1", "link_memories", projectScope, "fp-seed-link-d1-e1", "D1:E1"); err != nil {
		return nil, err
	}
	if err := commit(connection); err != nil {
		return nil, err
	}
	committed = true
	return map[string]any{"memories": []string{"D1", "E1", "E2"}, "relationship": "D1 -> justified_by -> E1"}, nil
}

func verifyFixture(connection *lbug.Connection) (any, error) {
	forward, err := query(connection, `MATCH (decision:Memory {id: "D1"})-[relationship:Evidence]->(evidence:Memory)
RETURN decision.id, decision.source, relationship.kind, relationship.source, evidence.id, evidence.source ORDER BY evidence.id;`, nil)
	if err != nil {
		return nil, err
	}
	reverse, err := query(connection, `MATCH (evidence:Memory {id: "E1"})<-[relationship:Evidence]-(decision:Memory)
RETURN evidence.id, evidence.source, relationship.kind, relationship.source, decision.id, decision.source ORDER BY decision.id;`, nil)
	if err != nil {
		return nil, err
	}
	if len(forward) != 1 || len(reverse) != 1 {
		return nil, fmt.Errorf("expected one forward and one reverse path, got %d and %d", len(forward), len(reverse))
	}
	expectedForward := []any{"D1", "Decision 12", "justified_by", "Review of Decision 12", "E1", "Test 7"}
	expectedReverse := []any{"E1", "Test 7", "justified_by", "Review of Decision 12", "D1", "Decision 12"}
	if !rowsEqual(forward[0], expectedForward) || !rowsEqual(reverse[0], expectedReverse) {
		return nil, fmt.Errorf("unexpected traversal results: forward=%v reverse=%v", forward, reverse)
	}
	return map[string]any{"forward": forward[0], "reverse": reverse[0]}, nil
}

func snapshotDatabase(connection *lbug.Connection) (any, error) {
	memories, err := query(connection, "MATCH (m:Memory) RETURN m.id, m.scope, m.text, m.source, m.recorded_at ORDER BY m.id;", nil)
	if err != nil {
		return nil, err
	}
	receipts, err := query(connection, "MATCH (r:Receipt) RETURN r.key, r.operation, r.scope, r.fingerprint, r.result_id ORDER BY r.key;", nil)
	if err != nil {
		return nil, err
	}
	relationships, err := query(connection, "MATCH (source:Memory)-[r:Evidence]->(target:Memory) RETURN source.id, target.id, r.kind, r.weight, r.source ORDER BY source.id, target.id;", nil)
	if err != nil {
		return nil, err
	}
	return map[string]any{"memories": memories, "receipts": receipts, "relationships": relationships}, nil
}

func runTransactions(args []string) error {
	flags := flag.NewFlagSet("transactions", flag.ContinueOnError)
	databasePath := flags.String("db", "", "database path")
	samples := flags.Int("samples", 50, "one-hop query samples")
	writes := flags.Int("writes", 20, "concurrent writes per connection")
	serializeWrites := flags.Bool("serialize-writes", true, "serialize writes through the daemon-style single-writer queue")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *databasePath == "" || *samples < 1 || *writes < 1 {
		return errors.New("--db, positive --samples, and positive --writes are required")
	}
	database, first, openDuration, err := openDatabase(*databasePath)
	if err != nil {
		return err
	}
	defer database.Close()
	defer first.Close()
	second, err := lbug.OpenConnection(database)
	if err != nil {
		return err
	}
	defer second.Close()

	result := transactionResult{OpenMicroseconds: openDuration.Microseconds(), WritesSerialized: *serializeWrites, WriterErrors: []string{}}
	if err := begin(first); err != nil {
		return err
	}
	if err := insertMemory(first, memory{ID: "TX-COMMIT", Scope: projectScope, Text: "Committed transaction fixture.", Source: "BB-0008"}); err != nil {
		_ = rollback(first)
		return err
	}
	if err := insertReceipt(first, "tx-commit", "record_memory", projectScope, "fp-tx-commit", "TX-COMMIT"); err != nil {
		_ = rollback(first)
		return err
	}
	result.UncommittedVisible, err = memoryExists(second, "TX-COMMIT")
	if err != nil {
		_ = rollback(first)
		return err
	}
	if err := commit(first); err != nil {
		return err
	}
	result.CommittedVisible, err = memoryExists(second, "TX-COMMIT")
	if err != nil {
		return err
	}

	if err := begin(first); err != nil {
		return err
	}
	if err := insertMemory(first, memory{ID: "TX-ROLLBACK", Scope: projectScope, Text: "Rolled back transaction fixture.", Source: "BB-0008"}); err != nil {
		_ = rollback(first)
		return err
	}
	if err := insertReceipt(first, "tx-rollback", "record_memory", projectScope, "fp-tx-rollback", "TX-ROLLBACK"); err != nil {
		_ = rollback(first)
		return err
	}
	if err := insertEvidence(first, "D1", "TX-ROLLBACK", "tested_by", 0.5, "BB-0008 rollback fixture"); err != nil {
		_ = rollback(first)
		return err
	}
	if err := rollback(first); err != nil {
		return err
	}
	result.RolledBackMemoryCount, err = count(first, "MATCH (m:Memory {id: $id}) RETURN count(m);", map[string]any{"id": "TX-ROLLBACK"})
	if err != nil {
		return err
	}
	result.RolledBackReceiptCount, err = count(first, "MATCH (r:Receipt {key: $key}) RETURN count(r);", map[string]any{"key": "tx-rollback"})
	if err != nil {
		return err
	}
	result.RolledBackRelationshipCount, err = count(first, "MATCH (:Memory {id: $source})-[r:Evidence]->(:Memory {id: $target}) RETURN count(r);", map[string]any{"source": "D1", "target": "TX-ROLLBACK"})
	if err != nil {
		return err
	}

	durations := make([]time.Duration, 0, *samples)
	for range *samples {
		started := time.Now()
		rows, queryErr := query(second, `MATCH (:Memory {id: "D1"})-[relationship:Evidence]->(evidence:Memory)
RETURN relationship.kind, evidence.id, evidence.source;`, nil)
		if queryErr != nil {
			return queryErr
		}
		if len(rows) != 1 {
			return fmt.Errorf("one-hop query returned %d rows", len(rows))
		}
		durations = append(durations, time.Since(started))
	}
	result.OneHopQuery = summarize(durations)

	var waitGroup sync.WaitGroup
	var resultLock sync.Mutex
	var writeLock sync.Mutex
	queueDurations := make([]time.Duration, 0, *writes*2)
	for worker, connection := range []*lbug.Connection{first, second} {
		waitGroup.Add(1)
		go func(worker int, connection *lbug.Connection) {
			defer waitGroup.Done()
			for index := range *writes {
				id := fmt.Sprintf("CW-%d-%03d", worker, index)
				key := "concurrent-" + id
				fp := fingerprint("record_memory", projectScope, id)
				queueStarted := time.Now()
				queued := false
				if *serializeWrites {
					if !writeLock.TryLock() {
						queued = true
						writeLock.Lock()
					}
				}
				queueDuration := time.Since(queueStarted)
				attempts, writeErr := writeWithRetry(connection, key, fp, memory{ID: id, Scope: projectScope, Text: "Concurrent transaction fixture.", Source: "BB-0008"}, false)
				if *serializeWrites {
					writeLock.Unlock()
				}
				resultLock.Lock()
				queueDurations = append(queueDurations, queueDuration)
				if queued {
					result.WriterQueueWaits++
				}
				result.ContentionRetries += attempts - 1
				if writeErr != nil {
					result.WriterErrors = append(result.WriterErrors, writeErr.Error())
				} else {
					result.ConcurrentWrites++
				}
				resultLock.Unlock()
			}
		}(worker, connection)
	}
	waitGroup.Wait()
	result.WriterQueueLatency = summarize(queueDurations)

	if result.UncommittedVisible || !result.CommittedVisible || result.RolledBackMemoryCount != 0 || result.RolledBackReceiptCount != 0 || result.RolledBackRelationshipCount != 0 || len(result.WriterErrors) != 0 {
		return fmt.Errorf("transaction acceptance check failed: %+v", result)
	}
	writeOutput(output{Command: "transactions", OK: true, Result: result})
	return nil
}

func runUnsafeConcurrentReads(args []string) error {
	flags := flag.NewFlagSet("unsafe-concurrent-reads", flag.ContinueOnError)
	databasePath := flags.String("db", "", "database path")
	reads := flags.Int("reads", 100, "reads per connection")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *databasePath == "" || *reads < 1 {
		return errors.New("--db and a positive --reads value are required")
	}
	database, first, _, err := openDatabase(*databasePath)
	if err != nil {
		return err
	}
	defer database.Close()
	defer first.Close()
	second, err := lbug.OpenConnection(database)
	if err != nil {
		return err
	}
	defer second.Close()

	var waitGroup sync.WaitGroup
	var resultLock sync.Mutex
	errorsFound := make([]string, 0)
	start := make(chan struct{})
	for _, connection := range []*lbug.Connection{first, second} {
		waitGroup.Add(1)
		go func(connection *lbug.Connection) {
			defer waitGroup.Done()
			<-start
			for range *reads {
				rows, queryErr := query(connection, "MATCH (m:Memory {id: $id}) RETURN m.source;", map[string]any{"id": "D1"})
				if queryErr != nil || len(rows) != 1 {
					resultLock.Lock()
					errorsFound = append(errorsFound, fmt.Sprintf("rows=%v error=%v", rows, queryErr))
					resultLock.Unlock()
					return
				}
			}
		}(connection)
	}
	close(start)
	waitGroup.Wait()
	if len(errorsFound) != 0 {
		return fmt.Errorf("concurrent read errors: %v", errorsFound)
	}
	writeOutput(output{Command: "unsafe-concurrent-reads", OK: true, Result: map[string]int{"reads": *reads * 2}})
	return nil
}

func runCrashWrite(args []string) error {
	flags := flag.NewFlagSet("crash-write", flag.ContinueOnError)
	databasePath := flags.String("db", "", "database path")
	phase := flags.String("phase", "", "before-commit or after-commit")
	key := flags.String("key", "", "idempotency key")
	id := flags.String("id", "", "memory ID")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *databasePath == "" || *key == "" || *id == "" || (*phase != "before-commit" && *phase != "after-commit") {
		return errors.New("--db, --key, --id, and a valid --phase are required")
	}
	database, connection, _, err := openDatabase(*databasePath)
	if err != nil {
		return err
	}
	_ = database
	if err := begin(connection); err != nil {
		return err
	}
	if err := insertMemory(connection, memory{ID: *id, Scope: projectScope, Text: "Crash recovery fixture.", Source: "BB-0009"}); err != nil {
		return err
	}
	if err := insertReceipt(connection, *key, "record_memory", projectScope, fingerprint("record_memory", projectScope, *id), *id); err != nil {
		return err
	}
	if err := insertEvidence(connection, "D1", *id, "tested_by", 0.75, "BB-0009 crash fixture"); err != nil {
		return err
	}
	if *phase == "after-commit" {
		if err := commit(connection); err != nil {
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "intentional crash at %s\n", *phase)
	_ = os.Stderr.Sync()
	os.Exit(crashExitCode)
	return nil
}

func runRetryWrite(args []string) error {
	flags := flag.NewFlagSet("retry-write", flag.ContinueOnError)
	databasePath := flags.String("db", "", "database path")
	key := flags.String("key", "", "idempotency key")
	id := flags.String("id", "", "memory ID")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *databasePath == "" || *key == "" || *id == "" {
		return errors.New("--db, --key, and --id are required")
	}
	database, connection, _, err := openDatabase(*databasePath)
	if err != nil {
		return err
	}
	defer database.Close()
	defer connection.Close()
	result, err := writeMemoryIdempotent(connection, *key, fingerprint("record_memory", projectScope, *id), memory{ID: *id, Scope: projectScope, Text: "Crash recovery fixture.", Source: "BB-0009"}, true)
	if err != nil {
		return err
	}
	writeOutput(output{Command: "retry-write", OK: true, Result: result})
	return nil
}

func runInspectWrite(args []string) error {
	flags := flag.NewFlagSet("inspect-write", flag.ContinueOnError)
	databasePath := flags.String("db", "", "database path")
	key := flags.String("key", "", "idempotency key")
	id := flags.String("id", "", "memory ID")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *databasePath == "" || *key == "" || *id == "" {
		return errors.New("--db, --key, and --id are required")
	}
	database, connection, _, err := openDatabase(*databasePath)
	if err != nil {
		return err
	}
	defer database.Close()
	defer connection.Close()
	memoryCount, err := count(connection, "MATCH (m:Memory {id: $id}) RETURN count(m);", map[string]any{"id": *id})
	if err != nil {
		return err
	}
	receiptCount, err := count(connection, "MATCH (r:Receipt {key: $key}) RETURN count(r);", map[string]any{"key": *key})
	if err != nil {
		return err
	}
	relationshipCount, err := count(connection, "MATCH (:Memory {id: $source})-[r:Evidence]->(:Memory {id: $target}) RETURN count(r);", map[string]any{"source": "D1", "target": *id})
	if err != nil {
		return err
	}
	writeOutput(output{Command: "inspect-write", OK: true, Result: map[string]int64{"memory_count": memoryCount, "receipt_count": receiptCount, "relationship_count": relationshipCount}})
	return nil
}

func runImport(args []string) error {
	flags := flag.NewFlagSet("import", flag.ContinueOnError)
	databasePath := flags.String("db", "", "database path")
	exportPath := flags.String("export", "", "export directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *databasePath == "" || *exportPath == "" {
		return errors.New("--db and --export are required")
	}
	return withDatabaseCommand("import", []string{"--db", *databasePath}, func(connection *lbug.Connection) (any, error) {
		_, err := query(connection, "IMPORT DATABASE "+cypherString(*exportPath)+";", nil)
		return map[string]string{"export": *exportPath}, err
	})
}

func writeWithRetry(connection *lbug.Connection, key, fp string, item memory, link bool) (int, error) {
	var lastError error
	for attempt := 1; attempt <= 8; attempt++ {
		_, lastError = writeMemoryIdempotent(connection, key, fp, item, link)
		if lastError == nil {
			return attempt, nil
		}
		time.Sleep(time.Duration(attempt) * time.Millisecond)
	}
	return 8, lastError
}

func writeMemoryIdempotent(connection *lbug.Connection, key, fp string, item memory, link bool) (writeResult, error) {
	rows, err := query(connection, "MATCH (r:Receipt {key: $key}) RETURN r.fingerprint, r.result_id;", map[string]any{"key": key})
	if err != nil {
		return writeResult{}, err
	}
	if len(rows) > 0 {
		storedFingerprint := fmt.Sprint(rows[0][0])
		if storedFingerprint != fp {
			return writeResult{}, errors.New("IDEMPOTENCY_CONFLICT")
		}
		return writeResult{ID: fmt.Sprint(rows[0][1]), Replayed: true}, nil
	}
	if err := begin(connection); err != nil {
		return writeResult{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = rollback(connection)
		}
	}()
	if err := insertMemory(connection, item); err != nil {
		return writeResult{}, err
	}
	if err := insertReceipt(connection, key, "record_memory", item.Scope, fp, item.ID); err != nil {
		return writeResult{}, err
	}
	if link {
		if err := insertEvidence(connection, "D1", item.ID, "tested_by", 0.75, "BB-0009 crash fixture"); err != nil {
			return writeResult{}, err
		}
	}
	if err := commit(connection); err != nil {
		return writeResult{}, err
	}
	committed = true
	return writeResult{ID: item.ID}, nil
}

func insertMemory(connection *lbug.Connection, item memory) error {
	_, err := query(connection, `CREATE (:Memory {id: $id, scope: $scope, text: $text, source: $source, recorded_at: $recordedAt});`, map[string]any{
		"id": item.ID, "scope": item.Scope, "text": item.Text, "source": item.Source, "recordedAt": "2026-11-24T00:00:00Z",
	})
	return err
}

func insertReceipt(connection *lbug.Connection, key, operation, scope, fp, resultID string) error {
	_, err := query(connection, `CREATE (:Receipt {key: $key, operation: $operation, scope: $scope, fingerprint: $fingerprint, result_id: $resultID});`, map[string]any{
		"key": key, "operation": operation, "scope": scope, "fingerprint": fp, "resultID": resultID,
	})
	return err
}

func insertEvidence(connection *lbug.Connection, sourceID, targetID, kind string, weight float64, source string) error {
	_, err := query(connection, `MATCH (source:Memory {id: $sourceID}), (target:Memory {id: $targetID})
CREATE (source)-[:Evidence {kind: $kind, weight: $weight, source: $source}]->(target);`, map[string]any{
		"sourceID": sourceID, "targetID": targetID, "kind": kind, "weight": weight, "source": source,
	})
	return err
}

func begin(connection *lbug.Connection) error {
	_, err := query(connection, "BEGIN TRANSACTION;", nil)
	return err
}

func commit(connection *lbug.Connection) error {
	_, err := query(connection, "COMMIT;", nil)
	return err
}

func rollback(connection *lbug.Connection) error {
	_, err := query(connection, "ROLLBACK;", nil)
	return err
}

func memoryExists(connection *lbug.Connection, id string) (bool, error) {
	value, err := count(connection, "MATCH (m:Memory {id: $id}) RETURN count(m);", map[string]any{"id": id})
	return value > 0, err
}

func count(connection *lbug.Connection, statement string, arguments map[string]any) (int64, error) {
	rows, err := query(connection, statement, arguments)
	if err != nil {
		return 0, err
	}
	if len(rows) != 1 || len(rows[0]) != 1 {
		return 0, fmt.Errorf("expected one count value, got %v", rows)
	}
	value, ok := rows[0][0].(int64)
	if !ok {
		return 0, fmt.Errorf("expected INT64 count, got %T", rows[0][0])
	}
	return value, nil
}

func query(connection *lbug.Connection, statement string, arguments map[string]any) ([][]any, error) {
	var result *lbug.QueryResult
	var err error
	if len(arguments) == 0 {
		result, err = connection.Query(statement)
	} else {
		prepared, prepareErr := connection.Prepare(statement)
		if prepareErr != nil {
			return nil, prepareErr
		}
		defer prepared.Close()
		result, err = connection.Execute(prepared, arguments)
	}
	if err != nil {
		return nil, err
	}
	defer result.Close()
	rows := make([][]any, 0, result.GetNumberOfRows())
	for result.HasNext() {
		tuple, nextErr := result.Next()
		if nextErr != nil {
			return nil, nextErr
		}
		values, valuesErr := tuple.GetAsSlice()
		if valuesErr != nil {
			return nil, valuesErr
		}
		rows = append(rows, values)
	}
	return rows, nil
}

func rowsEqual(actual, expected []any) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index := range actual {
		if fmt.Sprint(actual[index]) != fmt.Sprint(expected[index]) {
			return false
		}
	}
	return true
}

func fingerprint(parts ...string) string {
	hash := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(hash[:])
}

func summarize(durations []time.Duration) queryTiming {
	sort.Slice(durations, func(left, right int) bool { return durations[left] < durations[right] })
	median := durations[len(durations)/2]
	p95Index := (len(durations)*95 + 99) / 100
	if p95Index > len(durations) {
		p95Index = len(durations)
	}
	return queryTiming{MedianMicroseconds: median.Microseconds(), P95Microseconds: durations[p95Index-1].Microseconds(), Samples: len(durations)}
}

func cypherString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "\\'") + "'"
}

func writeOutput(value output) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		fatal(err.Error())
	}
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(2)
}

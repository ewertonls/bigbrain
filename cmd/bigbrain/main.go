package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ewertonls/bigbrain/internal/daemon"
	"github.com/ewertonls/bigbrain/internal/protocol"
)

const overview = `Usage: bigbrain [flags] COMMAND

Commands:
  record           Store a memory with its source.
  import           Import Markdown files from a directory.
  link             Connect memories within a scope or between a project and global.
  consolidate      Propose a local lesson from project evidence.
  publish          Publish the approved text and source of a project lesson.
  feedback         Record use of a relationship in a project.
  forget           Lower a relationship's priority based on its last use.
  review-weight    Approve a change to a global relationship's priority.
  feedback-history Inspect recorded uses and priority changes for a relationship.
  reconcile        Review two claims and record a project conclusion.
  query            Search memories or look up an ID.
  graph            Show a query's memories and paths as standalone HTML.
  mcp              Serve memory tools to an MCP client over standard I/O.
  invalidate       Hide a memory and preserve its history.
  history          Inspect a memory and its visible relationships.
  delete           Delete a memory and its incident relationships.
  context          Retrieve bounded, source-linked context.
  plugin doctor    Inspect the Codex binding and local runtime.
  db-path          Print the absolute path to the shared database file.
  project init ID  Save the project ID for this directory or Git repository.
  status           Show the daemon state.
  shutdown         Stop the daemon.
  help [COMMAND]   Show general or command-specific help.

Run bigbrain COMMAND -h for command flags.
`

type options struct {
	db   string
	json bool
}

type scopeOptions struct {
	project    string
	current    bool
	global     bool
	readGlobal *bool
}

type projectResult struct {
	ID string `json:"id"`
}

type databaseLocation struct {
	Scope        string `json:"scope"`
	Directory    string `json:"directory"`
	DatabaseFile string `json:"database_file"`
}

// Some commands write their own format directly to stdout.
type streamOutput func(io.Writer) error

func main() {
	if len(os.Args) > 1 && os.Args[1] == "__daemon" {
		if err := runDaemon(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		os.Exit(1)
	}
}

func runDaemon(args []string) error {
	flags := newFlags("daemon")
	path := flags.String("db", "", "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *path == "" || flags.NArg() != 0 {
		return invalid("Daemon mode requires a database directory.")
	}
	return daemon.Serve(*path)
}

func run(args []string, stdout, stderr io.Writer) error {
	var options options
	result, err := execute(args, &options)
	if stream, ok := result.(streamOutput); ok {
		if err := stream(stdout); err != nil {
			fmt.Fprintln(stderr, err)
			return err
		}
		return nil
	}
	if options.json {
		if outputErr := protocol.Encode(stdout, result, err); outputErr != nil {
			fmt.Fprintln(stderr, outputErr)
			return outputErr
		}
		return err
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return err
	}
	switch value := result.(type) {
	case string:
		_, err = fmt.Fprint(stdout, value)
	case protocol.WriteResult:
		_, err = fmt.Fprintln(stdout, value.ID)
	case importResult:
		_, err = fmt.Fprintf(stdout, "Markdown files: %d\nMemories created: %d\nMemories already imported: %d\n", value.Files, value.Created, value.Replayed)
	case protocol.StatusResult:
		_, err = fmt.Fprintln(stdout, value.State)
	case databaseLocation:
		_, err = fmt.Fprintln(stdout, value.DatabaseFile)
	default:
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		err = encoder.Encode(value)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
	}
	return err
}

func execute(args []string, options *options) (any, error) {
	global := newFlags("bigbrain")
	options.register(global)
	if err := options.parse(global, args); err != nil {
		return parseError(global, err)
	}
	if global.NArg() == 0 {
		return helpText(global), nil
	}
	command := global.Arg(0)
	args = global.Args()[1:]
	if command == "help" {
		// Parse global flags on either side of the help topic. Dispatch only
		// the help flag to the chosen command, never a data operation.
		if err := options.parse(global, args); err != nil {
			return parseError(global, err)
		}
		if global.NArg() == 0 {
			return helpText(global), nil
		}
		command, args = global.Arg(0), global.Args()[1:]
		var topic []string
		if len(args) > 0 && ((command == "project" && args[0] == "init") || (command == "plugin" && args[0] == "doctor")) {
			topic, args = args[:1], args[1:]
		}
		if err := options.parseFlags(global, args); err != nil {
			return parseError(global, err)
		}
		args = append(topic, "-h")
	}
	flags := newFlags(command)
	options.register(flags)
	switch command {
	case "help":
		return helpText(flags), nil
	case "context":
		return contextCommand(options, args)
	case "plugin":
		return pluginCommand(options, args)
	case "mcp":
		if err := options.parseFlags(flags, args); err != nil {
			return parseError(flags, err)
		}
		return streamOutput(func(w io.Writer) error {
			return serveMCP(options.db, os.Stdin, w)
		}), nil
	case "project":
		if err := options.parse(flags, args); err != nil {
			return parseError(flags, err)
		}
		if flags.Arg(0) == "init" {
			args = flags.Args()[1:]
			flags = newFlags("project init")
			options.register(flags)
			if err := options.parse(flags, args); err != nil {
				return parseError(flags, err)
			}
			if flags.NArg() == 1 {
				return initProject(flags.Arg(0))
			}
		}
		return nil, invalid("Run bigbrain project init ID to configure a project.")
	case "status", "shutdown":
		if err := options.parseFlags(flags, args); err != nil {
			return parseError(flags, err)
		}
		return lifecycle(command, options.db)
	}

	scope := scopeOptions{}
	flags.StringVar(&scope.project, "project", "", "select project `ID`")
	flags.BoolVar(&scope.current, "current-project", false, "use the project configured for this directory or Git worktree")
	globalUsage := "select the global collection"
	if projectOnlyCommand(command) {
		globalUsage = "unavailable for this project operation"
	}
	flags.BoolVar(&scope.global, "global", false, globalUsage)

	switch command {
	case "import":
		return importCommand(flags, args, &scope, options)
	case "db-path":
		if err := options.parseFlags(flags, args); err != nil {
			return parseError(flags, err)
		}
		selected, err := scope.resolve(flags)
		if err != nil {
			return nil, err
		}
		return locateDatabase(selected, options.db)
	case "record":
		var request protocol.RecordRequest
		flags.StringVar(&request.Type, "type", "", "semantic, episodic, procedural, decision, preference, entity or observation")
		flags.StringVar(&request.Key, "key", "", "idempotency key (required)")
		flags.StringVar(&request.Text, "text", "", "memory text (required)")
		flags.StringVar(&request.Source, "source", "", "memory source (required)")
		return dataCommand[protocol.WriteResult](flags, args, &scope, options, &request.Scope, &request)
	case "link":
		var request protocol.LinkRequest
		linkFlags(flags, &request)
		return dataCommand[protocol.WriteResult](flags, args, &scope, options, &request.Scope, &request)
	case "consolidate":
		var request protocol.ConsolidateRequest
		consolidateFlags(flags, &request)
		return dataCommand[protocol.WriteResult](flags, args, &scope, options, &request.Scope, &request)
	case "publish":
		var request protocol.PublishRequest
		flags.StringVar(&request.Key, "key", "", "idempotency key (required)")
		flags.StringVar(&request.ProposalID, "proposal", "", "active project proposal ID")
		flags.StringVar(&request.Text, "text", "", "exact text approved for global publication")
		flags.StringVar(&request.Source, "source", "", "approved public source reference")
		flags.BoolVar(&request.Approve, "approve", false, "approve publication of the supplied text and source")
		return dataCommand[protocol.WriteResult](flags, args, &scope, options, &request.Scope, &request)
	case "feedback":
		var request protocol.FeedbackRequest
		feedbackTargetFlags(flags, &request.FeedbackTarget)
		flags.StringVar(&request.EvidenceID, "evidence", "", "supporting memory ID (required for confidence changes)")
		optionalFloatFlag(flags, "confidence", &request.Confidence, "revised confidence from 0 to 1")
		return dataCommand[protocol.WriteResult](flags, args, &scope, options, &request.Scope, &request)
	case "forget":
		var request protocol.ForgetRequest
		feedbackTargetFlags(flags, &request.FeedbackTarget)
		flags.StringVar(&request.Before, "before", "", "halve the weight if the last use or creation predates this RFC 3339 timestamp")
		return dataCommand[protocol.WriteResult](flags, args, &scope, options, &request.Scope, &request)
	case "review-weight":
		var request protocol.ReviewWeightRequest
		feedbackTargetFlags(flags, &request.FeedbackTarget)
		optionalFloatFlag(flags, "weight", &request.Weight, "approved nonnegative global weight")
		flags.BoolVar(&request.Approve, "approve", false, "approve this global weight change")
		return dataCommand[protocol.WriteResult](flags, args, &scope, options, &request.Scope, &request)
	case "feedback-history":
		var request protocol.FeedbackHistoryRequest
		flags.StringVar(&request.RelationshipID, "relationship", "", "relationship ID")
		scope.readGlobalFlag(flags, &request.IncludeGlobal)
		flags.IntVar(&request.Limit, "limit", protocol.DefaultQueryLimit, "maximum events per page")
		flags.StringVar(&request.After, "after", "", "event cursor from the previous page")
		return dataCommand[protocol.FeedbackHistoryResult](flags, args, &scope, options, &request.Scope, &request)
	case "reconcile":
		var request protocol.ReconcileRequest
		reconcileFlags(flags, &request)
		return dataCommand[protocol.WriteResult](flags, args, &scope, options, &request.Scope, &request)
	case "invalidate":
		var request protocol.InvalidateRequest
		memoryWriteFlags(flags, &request.MemoryWriteRequest)
		flags.StringVar(&request.Reason, "reason", "", "reason for invalidation (required)")
		flags.StringVar(&request.Source, "source", "", "source of invalidation (required)")
		return dataCommand[protocol.WriteResult](flags, args, &scope, options, &request.Scope, &request)
	case "delete":
		var request protocol.MemoryWriteRequest
		memoryWriteFlags(flags, &request)
		return dataCommand[protocol.WriteResult](flags, args, &scope, options, &request.Scope, &request)
	case "history":
		var request protocol.HistoryRequest
		flags.StringVar(&request.ID, "id", "", "memory ID (required)")
		scope.readGlobalFlag(flags, &request.IncludeGlobal)
		flags.IntVar(&request.Limit, "limit", protocol.DefaultQueryLimit, "maximum related records per page")
		flags.StringVar(&request.After, "after", "", "relationship cursor from the previous page")
		return dataCommand[protocol.HistoryResult](flags, args, &scope, options, &request.Scope, &request)
	case "query", "graph":
		var request protocol.QueryRequest
		flags.StringVar(&request.Text, "text", "", "search text (use either -text or -id)")
		flags.StringVar(&request.ID, "id", "", "memory ID")
		flags.StringVar(&request.Mode, "mode", "hybrid", "retrieval mode: hybrid, text or associative")
		flags.IntVar(&request.Limit, "limit", protocol.DefaultQueryLimit, "maximum results")
		scope.readGlobalFlag(flags, &request.IncludeGlobal)
		optionalIntFlag(flags, "max-depth", &request.MaxDepth,
			fmt.Sprintf("maximum graph hops (1-%d, default %d)", protocol.MaxGraphDepth, protocol.DefaultMaxDepth))
		optionalIntFlag(flags, "max-neighbors", &request.MaxNeighbors,
			fmt.Sprintf("maximum neighbors per node (1-%d, default %d)", protocol.MaxGraphNeighbors, protocol.DefaultMaxNeighbors))
		path, err := options.parseDataRequest(flags, args, &scope, &request.Scope, &request)
		if err != nil {
			return parseError(flags, err)
		}
		matches, err := callDaemon[protocol.QueryResult](context.Background(), path, "query", request)
		if err != nil {
			return nil, err
		}
		if command == "graph" && !options.json {
			return streamOutput(func(w io.Writer) error {
				return writeGraph(w, request, matches)
			}), nil
		}
		return matches, nil
	default:
		options.json = requestsJSON(flags, args, options.json)
		return nil, invalid("Unknown command. Run bigbrain help for usage.")
	}
}

func optionalIntFlag(flags *flag.FlagSet, name string, dest **int, usage string) {
	flags.Func(name, usage, func(value string) error {
		n, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		*dest = &n
		return nil
	})
}

func feedbackTargetFlags(flags *flag.FlagSet, r *protocol.FeedbackTarget) {
	flags.StringVar(&r.Key, "key", "", "idempotency key (required)")
	flags.StringVar(&r.RelationshipID, "relationship", "", "relationship ID")
	flags.StringVar(&r.Author, "author", "", "person or agent responsible for this write")
	flags.StringVar(&r.Source, "source", "", "source reference for this write")
	if flags.Name() != "review-weight" {
		flags.BoolVar(&r.IncludeGlobal, "include-global", false, "allow visible global records for this project write")
	}
}

func reconcileFlags(flags *flag.FlagSet, r *protocol.ReconcileRequest) {
	flags.StringVar(&r.Key, "key", "", "idempotency key (required)")
	flags.StringVar(&r.SupportingID, "supporting", "", "supporting claim ID")
	flags.StringVar(&r.ContraryID, "contrary", "", "contrary claim ID")
	flags.StringVar(&r.Outcome, "outcome", "", "different_contexts, disputed or supersedes")
	flags.StringVar(&r.SupersedingSide, "superseding-side", "", "claim that supersedes the other: supporting or contrary")
	flags.StringVar(&r.Text, "text", "", "review conclusion")
	flags.StringVar(&r.AppliesWhen, "applies-when", "", "conditions where the conclusion applies")
	flags.StringVar(&r.Rationale, "rationale", "", "why the claims support this conclusion")
	flags.StringVar(&r.Author, "author", "", "reviewer")
	flags.StringVar(&r.Source, "source", "", "review source reference")
	flags.BoolVar(&r.IncludeGlobal, "include-global", false, "allow global claims")
}

func optionalFloatFlag(flags *flag.FlagSet, name string, dest **float64, usage string) {
	flags.Func(name, usage, func(value string) error {
		number, err := strconv.ParseFloat(value, 64)
		if err == nil {
			*dest = &number
		}
		return err
	})
}

func linkFlags(flags *flag.FlagSet, request *protocol.LinkRequest) {
	flags.StringVar(&request.Key, "key", "", "idempotency key (required)")
	flags.StringVar(&request.FromID, "from", "", "source memory `ID` (required)")
	flags.StringVar(&request.ToID, "to", "", "target memory `ID` (required)")
	flags.StringVar(&request.Kind, "type", "", "relationship type (required)")
	flags.StringVar(&request.Source, "source", "", "relationship provenance (required)")
	flags.Float64Var(&request.Weight, "weight", 1, "relationship priority")
	optionalFloatFlag(flags, "confidence", &request.Confidence, "strength of the evidence from 0 to 1 (omit if unknown)")
}

func memoryWriteFlags(flags *flag.FlagSet, request *protocol.MemoryWriteRequest) {
	flags.StringVar(&request.Key, "key", "", "idempotency key (required)")
	flags.StringVar(&request.ID, "id", "", "memory ID (required)")
}

func consolidateFlags(flags *flag.FlagSet, request *protocol.ConsolidateRequest) {
	flags.StringVar(&request.Key, "key", "", "idempotency key (required)")
	flags.Func("evidence", "project memory `ID` (required; repeat for each source)", func(id string) error {
		request.EvidenceIDs = append(request.EvidenceIDs, id)
		return nil
	})
	var draft protocol.ExternalProposal
	draftFlag := func(field *string, name, usage string) {
		flags.Func(name, usage, func(value string) error {
			*field = value
			request.ExternalProposal = &draft
			return nil
		})
	}
	draftFlag(&draft.Text, "text", "externally proposed lesson")
	draftFlag(&draft.AppliesWhen, "applies-when", "when the external lesson applies")
	draftFlag(&draft.Rationale, "rationale", "why the evidence supports the external lesson")
	draftFlag(&draft.Producer, "producer", "person or agent that produced the external lesson")
	draftFlag(&draft.Source, "source", "external proposal source (defaults to producer)")
}

func dataCommand[R any](flags *flag.FlagSet, args []string, selection *scopeOptions,
	options *options, scope *string, request interface{ Validate() error }) (any, error) {
	path, err := options.parseDataRequest(flags, args, selection, scope, request)
	if err != nil {
		return parseError(flags, err)
	}
	return callDaemon[R](context.Background(), path, flags.Name(), request)
}

func (o *options) parseDataRequest(flags *flag.FlagSet, args []string, selection *scopeOptions,
	scope *string, request interface{ Validate() error }) (string, error) {
	if err := o.parseFlags(flags, args); err != nil {
		return "", err
	}
	var err error
	*scope, err = selection.resolve(flags)
	if err != nil {
		return "", err
	}
	if err := request.Validate(); err != nil {
		return "", err
	}
	return databasePath(o.db)
}

func callDaemon[R any](ctx context.Context, path, operation string, request any) (R, error) {
	var result R
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	client, err := daemon.Connect(ctx, path, true)
	if err == nil {
		err = client.Call(ctx, operation, request, &result)
	}
	return result, err
}

func lifecycle(command, db string) (any, error) {
	path, err := databasePath(db)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	stopped := protocol.StatusResult{State: "stopped"}
	if command == "shutdown" {
		return stopped, daemon.Shutdown(ctx, path)
	}
	client, err := daemon.Connect(ctx, path, false)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return stopped, nil
	}
	return client.Status(), nil
}

func databasePath(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	base := os.Getenv("XDG_DATA_HOME")
	if !filepath.IsAbs(base) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", protocol.NewError("DATABASE_UNAVAILABLE", "Set --db or configure a home directory.")
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "bigbrain"), nil
}

func locateDatabase(scope, override string) (databaseLocation, error) {
	dir, err := databasePath(override)
	if err != nil {
		return databaseLocation{}, err
	}
	// Match the daemon's physical directory when it already exists. A fresh
	// checkout can also inspect its configured path before the first write.
	if physical, err := filepath.EvalSymlinks(dir); err == nil {
		dir = physical
	} else if !errors.Is(err, os.ErrNotExist) {
		return databaseLocation{}, protocol.NewError("DATABASE_UNAVAILABLE", err.Error())
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return databaseLocation{}, protocol.NewError("DATABASE_UNAVAILABLE", err.Error())
	}
	return databaseLocation{Scope: scope, Directory: dir, DatabaseFile: daemon.DatabaseFile(dir)}, nil
}

func newFlags(command string) *flag.FlagSet {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	return flags
}

func (o *options) register(flags *flag.FlagSet) {
	// StringVar and BoolVar initialize their destinations. Preserve values
	// parsed before the command while documenting the CLI defaults.
	inherited := *o
	flags.StringVar(&o.db, "db", "", "use database directory `DIR`")
	flags.BoolVar(&o.json, "json", false, "print a versioned JSON response")
	*o = inherited
}

func (o *options) parse(flags *flag.FlagSet, args []string) error {
	err := flags.Parse(args)
	if err != nil {
		o.json = requestsJSON(flags, args, o.json)
	}
	return err
}

func (o *options) parseFlags(flags *flag.FlagSet, args []string) error {
	if err := o.parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		o.json = requestsJSON(flags, flags.Args(), o.json)
		return invalid("Unexpected positional argument. Run bigbrain help for usage.")
	}
	return nil
}

func helpText(flags *flag.FlagSet) string {
	var out strings.Builder
	switch flags.Name() {
	case "bigbrain":
		out.WriteString(overview)
	case "project":
		out.WriteString("Usage: bigbrain project [flags] init ID\n")
	case "project init":
		out.WriteString("Usage: bigbrain project init [flags] ID\n")
	case "import":
		out.WriteString("Usage: bigbrain import [flags] DIR\n\nRecursively record Markdown sections as semantic memories.\n")
	case "help":
		out.WriteString("Usage: bigbrain help [flags] [COMMAND]\n")
	default:
		fmt.Fprintf(&out, "Usage: bigbrain %s [flags]\n", flags.Name())
	}
	if projectOnlyCommand(flags.Name()) {
		out.WriteString("\nUse the configured project by default, or select -project ID.\n")
		if flags.Name() == "consolidate" {
			out.WriteString("For an external proposal, supply -text, -applies-when, -rationale, and -producer together.\n")
		}
	} else if flags.Name() == "review-weight" {
		out.WriteString("\nChoose -global for this reviewed write.\n")
	} else if flags.Lookup("project") != nil {
		out.WriteString("\nUse the configured project by default. Select a scope with -project ID, -current-project, or -global.\n")
	}
	out.WriteString("\nFlags:\n")
	output := flags.Output()
	flags.SetOutput(&out)
	flags.PrintDefaults()
	flags.SetOutput(output)
	return out.String()
}

func parseError(flags *flag.FlagSet, err error) (any, error) {
	if errors.Is(err, flag.ErrHelp) {
		return helpText(flags), nil
	}
	var apiErr *protocol.Error
	if errors.As(err, &apiErr) {
		return nil, apiErr
	}
	return nil, invalid(err.Error())
}

func invalid(message string) error { return protocol.NewError("INVALID_REQUEST", message) }

// The flag parser stops at the first bad argument. Preserve JSON output when
// --json follows that argument, while skipping values of known flags.
func requestsJSON(flags *flag.FlagSet, args []string, requested bool) bool {
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			break
		}
		option, hasDash := strings.CutPrefix(args[i], "-")
		if !hasDash {
			continue
		}
		name, value, assigned := strings.Cut(strings.TrimPrefix(option, "-"), "=")
		if name == "json" {
			if !assigned {
				requested = true
			} else if enabled, err := strconv.ParseBool(value); err == nil {
				requested = enabled
			}
			continue
		}
		if f := flags.Lookup(name); f != nil && !assigned {
			boolFlag, ok := f.Value.(interface{ IsBoolFlag() bool })
			if !ok || !boolFlag.IsBoolFlag() {
				i++
			}
		}
	}
	return requested
}

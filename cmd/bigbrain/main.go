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

const usage = `Usage: bigbrain [--db DIR] [--json] COMMAND

  record --key KEY --text TEXT --source SOURCE
  link   --key KEY --from ID --to ID --type TYPE [--weight NUMBER] --source SOURCE
  query  (--text TEXT | --id ID) [--mode text|associative] [--limit N] [--include-global]
  db-path    Print the absolute path to the shared database file.
  project init ID
  status
  shutdown
  help

Select exactly one scope on record, link, query, and db-path.
  --project ID       Select a project by ID.
  --current-project  Use the project ID saved by project init in Git.
  --global           Select the global collection.
`

type options struct {
	db   string
	json bool
}

type scopeOptions struct {
	project string
	current bool
	global  bool
}

type projectResult struct {
	ID string `json:"id"`
}

type databaseLocation struct {
	Scope        string `json:"scope"`
	Directory    string `json:"directory"`
	DatabaseFile string `json:"database_file"`
}

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
	case protocol.StatusResult:
		_, err = fmt.Fprintln(stdout, value.State)
	case databaseLocation:
		_, err = fmt.Fprintln(stdout, value.DatabaseFile)
	default:
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		err = encoder.Encode(value)
	}
	return err
}

func execute(args []string, options *options) (any, error) {
	global := newFlags("bigbrain")
	options.register(global)
	if err := options.parse(global, args); err != nil {
		return parseError(err)
	}
	if global.NArg() == 0 {
		return usage, nil
	}
	command := global.Arg(0)
	args = global.Args()[1:]
	flags := newFlags(command)
	options.register(flags)
	switch command {
	case "help":
		options.json = requestsJSON(flags, args, options.json)
		return usage, nil
	case "project":
		if err := options.parse(flags, args); err != nil {
			return parseError(err)
		}
		return projectCommand(flags.Args())
	case "status", "shutdown":
		if err := options.parseFlags(flags, args); err != nil {
			return parseError(err)
		}
		return lifecycle(command, options.db)
	}

	scope := scopeOptions{}
	flags.StringVar(&scope.project, "project", "", "project ID")
	flags.BoolVar(&scope.current, "current-project", false, "configured project")
	flags.BoolVar(&scope.global, "global", false, "global scope")

	switch command {
	case "db-path":
		if err := options.parseFlags(flags, args); err != nil {
			return parseError(err)
		}
		selected, err := scope.resolve(flags)
		if err != nil {
			return nil, err
		}
		return locateDatabase(selected, options.db)
	case "record":
		var request protocol.RecordRequest
		flags.StringVar(&request.Key, "key", "", "idempotency key")
		flags.StringVar(&request.Text, "text", "", "memory text")
		flags.StringVar(&request.Source, "source", "", "memory source")
		return dataCommand[protocol.WriteResult](flags, args, &scope, options, &request.Scope, &request)
	case "link":
		var request protocol.LinkRequest
		flags.StringVar(&request.Key, "key", "", "idempotency key")
		flags.StringVar(&request.FromID, "from", "", "stored source endpoint")
		flags.StringVar(&request.ToID, "to", "", "stored target endpoint")
		flags.StringVar(&request.Kind, "type", "", "relationship type")
		flags.StringVar(&request.Source, "source", "", "relationship provenance")
		flags.Float64Var(&request.Weight, "weight", 1, "relationship priority")
		return dataCommand[protocol.WriteResult](flags, args, &scope, options, &request.Scope, &request)
	case "query":
		var request protocol.QueryRequest
		flags.StringVar(&request.Text, "text", "", "search text")
		flags.StringVar(&request.ID, "id", "", "memory ID")
		flags.StringVar(&request.Mode, "mode", "associative", "retrieval mode")
		flags.IntVar(&request.Limit, "limit", protocol.DefaultQueryLimit, "maximum results")
		flags.BoolVar(&request.IncludeGlobal, "include-global", false, "include global records")
		return dataCommand[protocol.QueryResult](flags, args, &scope, options, &request.Scope, &request)
	default:
		options.json = requestsJSON(flags, args, options.json)
		return nil, invalid("Unknown command. Run bigbrain help for usage.")
	}
}

func dataCommand[R any](flags *flag.FlagSet, args []string, selection *scopeOptions,
	options *options, scope *string, request interface{ Validate() error }) (any, error) {
	if err := options.parseFlags(flags, args); err != nil {
		return parseError(err)
	}
	var err error
	*scope, err = selection.resolve(flags)
	if err != nil {
		return nil, err
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	path, err := databasePath(options.db)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := daemon.Connect(ctx, path, true)
	if err != nil {
		return nil, err
	}
	var result R
	if err := client.Call(ctx, flags.Name(), request, &result); err != nil {
		return nil, err
	}
	return result, nil
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
	flags.StringVar(&o.db, "db", o.db, "database directory")
	flags.BoolVar(&o.json, "json", o.json, "versioned JSON output")
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

func parseError(err error) (any, error) {
	if errors.Is(err, flag.ErrHelp) {
		return usage, nil
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

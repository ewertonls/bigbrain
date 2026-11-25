package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ewertonls/bigbrain/internal/daemon"
	"github.com/ewertonls/bigbrain/internal/markdown"
	"github.com/ewertonls/bigbrain/internal/protocol"
)

type importResult struct {
	Files    int `json:"files"`
	Created  int `json:"created"`
	Replayed int `json:"replayed"`
}

func importCommand(flags *flag.FlagSet, args []string, scope *scopeOptions, options *options) (any, error) {
	if err := options.parse(flags, args); err != nil {
		return parseError(flags, err)
	}
	if flags.NArg() == 0 {
		return nil, invalid("Supply a Markdown directory to import.")
	}
	dir := flags.Arg(0)
	if err := options.parseFlags(flags, flags.Args()[1:]); err != nil {
		return parseError(flags, err)
	}
	if dir == "" {
		return nil, invalid("Supply a Markdown directory to import.")
	}
	selected, err := scope.resolve(flags)
	if err != nil {
		return nil, err
	}
	db, err := databasePath(options.db)
	if err != nil {
		return nil, err
	}

	importer := markdownImporter{scope: selected, db: db}
	var failedFile string
	err = walkMarkdownFiles(dir, selected != "global", func(path, source string) error {
		err := importer.readFile(path, source)
		if err != nil {
			failedFile = source
		}
		return err
	})
	if err != nil {
		code, message := "IMPORT_FAILED", err.Error()
		var apiErr *protocol.Error
		if errors.As(err, &apiErr) {
			code, message = apiErr.Code, apiErr.Message
		}
		if failedFile != "" {
			message = fmt.Sprintf("%s: %s", failedFile, message)
		}
		return nil, protocol.NewError(code, fmt.Sprintf("%s (%d created, %d replayed before the error).", message, importer.result.Created, importer.result.Replayed))
	}
	return importer.result, nil
}

type markdownImporter struct {
	scope  string
	db     string
	client *daemon.Client
	result importResult
}

func (m *markdownImporter) readFile(path, source string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	lastLine, part := 0, 0
	err = markdown.ReadChunks(file, func(line int, text string) error {
		// A line longer than the memory limit can produce identical fragments.
		if line != lastLine {
			lastLine, part = line, 0
		}
		part++
		ref := fmt.Sprintf("%s#L%d", source, line)
		digest := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%d\x00%s", m.scope, ref, part, text)))
		return m.record(protocol.RecordRequest{
			Scope: m.scope, Type: "semantic", Source: ref, Text: text,
			Key: fmt.Sprintf("markdown:v1:%x", digest),
		})
	})
	if err == nil && lastLine != 0 {
		m.result.Files++
	}
	return err
}

func (m *markdownImporter) record(request protocol.RecordRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if m.client == nil {
		client, err := daemon.Connect(ctx, m.db, true)
		if err != nil {
			return err
		}
		m.client = client
	}
	var write protocol.WriteResult
	if err := m.client.Call(ctx, "record", request, &write); err != nil {
		return err
	}
	if write.Replayed {
		m.result.Replayed++
	} else {
		m.result.Created++
	}
	return nil
}

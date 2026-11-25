package main

import (
	"context"
	"encoding/json"
	"io"

	adapter "github.com/ewertonls/bigbrain/internal/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func serveMCP(db string, stdin io.ReadCloser, stdout io.Writer) error {
	path, err := databasePath(db)
	if err != nil {
		return err
	}
	server := adapter.NewServer(func(ctx context.Context, name string, request any) (json.RawMessage, error) {
		return callDaemon[json.RawMessage](ctx, path, name, request)
	})
	return server.Run(context.Background(), &mcp.IOTransport{Reader: stdin, Writer: mcpOutput{stdout}})
}

// The command's caller owns stdout.
type mcpOutput struct{ io.Writer }

func (mcpOutput) Close() error { return nil }

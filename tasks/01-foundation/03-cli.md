---
kadai: task
id: "BB-0014"
title: "Expose record, link, and query operations through the v1 contract"
status: "todo"
parent: BB-0002
depends_on: [BB-0013]
---
Implement the CLI over HTTP on the Unix socket, including automatic startup, shutdown, and checks for database identity and protocol version. Accept the caller’s idempotency key and document how to save the operation, scope, payload, and key before invoking the CLI.

## Acceptance criteria

In JSON mode, stdout contains only the response, with `version` and either `result` or `error`. Each error includes `code` and `message`. Query results include the ID, scope, text, source, and match reason. A version mismatch returns a stable error code and guidance. Help and status work while the daemon is stopped.

[Project reference](../../docs/SPEC.md)

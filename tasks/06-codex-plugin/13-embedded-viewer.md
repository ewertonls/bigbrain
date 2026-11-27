---
kadai: task
id: "BB-0048"
title: "Optional · Embed the shared viewer on verified MCP Apps hosts"
status: "todo"
parent: BB-0034
depends_on: [BB-0035, BB-0046, BB-0047]
---
Proceed only for a client that passed the rendered-resource and bridge round-trip in WP-01. Package a separate MCP Apps companion with the same frontend and helper state as the browser viewer. Reuse the [stdio adapter](../../internal/mcp/server.go) and [MCP entry point](../../cmd/bigbrain/mcp.go); keep the base plugin independent.

## Acceptance criteria

- On the recorded host build, render a `text/html;profile=mcp-app` resource and complete a real scoped request from the UI. Require an explicit validated binding on every operation even when the server starts in the plugin cache.
- Keep model-visible output compact and use verified app-only result metadata for bounded graph details. Validate authorization and scope independently of UI visibility. Gate host context actions on separately demonstrated support.
- Disable the companion and demonstrate the base CLI, hooks and all browser screens still work. Where rendering is unsupported, record the browser fallback and leave this extension outside the release criteria.

Reuse the [same screens and interaction contract](references/ui-specification.md).

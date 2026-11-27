---
kadai: task
id: "BB-0035"
title: "Verify Codex plugin discovery, hooks and embedded UI support"
status: "todo"
parent: BB-0034
---
Record a capability matrix for the intended local CLI and desktop surfaces. Capture the exact client build, OS/architecture, orchestration mode, install method and reproducible steps. Use a disposable minimal package with one discoverable skill and hook. Record PASS, FALLBACK or UNSUPPORTED for each capability.

## Acceptance criteria

- Observe hook behavior before and after host trust review. Capture the actual stdin fields and stdout shape for startup, resume and compact. Demonstrate that a marker returned by `SessionStart` with `source: compact` reaches the immediate continuation.
- Exercise one read-only stdio tool. To qualify an embedded host, render a minimal MCP Apps HTML resource, complete a UI-to-tool round-trip and verify the advertised app-only visibility and result metadata.
- Name the supported local hook profile and any limits on cloud-orchestrated work. Where embedding is unavailable, select the local browser viewer and keep base delivery actionable.

Reuse the [current MCP entry point](../../cmd/bigbrain/mcp.go) and the [existing Codex integration](../../integrations/codex/bigbrain-handoff/SKILL.md).

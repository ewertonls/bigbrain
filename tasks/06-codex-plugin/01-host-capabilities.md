---
kadai: task
id: "BB-0035"
title: "Verify Codex plugin discovery, hooks and embedded UI support"
status: "done"
parent: BB-0034
---
Record a capability matrix for the intended local CLI and desktop surfaces. Capture the exact client build, OS/architecture, orchestration mode, install method and reproducible steps. Use a disposable minimal package with one discoverable skill and hook. Record PASS, FALLBACK or UNSUPPORTED for each capability.

## Acceptance criteria

- Observe hook behavior before and after host trust review. Capture the actual stdin fields and stdout shape for startup, resume and compact. Demonstrate that a marker returned by `SessionStart` with `source: compact` reaches the immediate continuation.
- Exercise one read-only stdio tool. To qualify an embedded host, render a minimal MCP Apps HTML resource, complete a UI-to-tool round-trip and verify the advertised app-only visibility and result metadata.
- Name the supported local hook profile and any limits on cloud-orchestrated work. Where embedding is unavailable, select the local browser viewer and keep base delivery actionable.

Reuse the [current MCP entry point](../../cmd/bigbrain/mcp.go) and the [existing Codex integration](../../integrations/codex/bigbrain-handoff/SKILL.md).

## Verified host profile

| Capability | Local CLI 0.172.0 on macOS ARM64 27.2.1 |
| --- | --- |
| Local marketplace and skills | PASS. Install a fixture with `codex plugin marketplace add DIR` and `codex plugin add probe@bb-lab`. App-server `plugin/read` returns the enabled namespaced skill. |
| Command hook trust | PASS. Inspect `hooks/list` before and after reviewing the command through the host. The same command changed from untrusted to trusted. |
| Session continuity | PASS. From app-server, start a thread, complete a turn, reconnect and resume it, complete a second turn, then call `thread/compact/start`. The fixture recorded startup, resume and compact for the same session. |
| Context output | PASS. Return a marker in `hookSpecificOutput.additionalContext` with `hookEventName: SessionStart`. On the immediate turn after compact, the model reproduced the marker without tools or the value appearing in the user prompt. |
| MCP stdio | PASS. Initialize the BigBrain server and call the read-only query tool with an explicit test project. |
| Embedded UI | FALLBACK to the local browser. Resource rendering and app-only visibility have not been demonstrated in the desktop client. |

Observed stdin fields were `cwd`, `hook_event_name`, `model`, `permission_mode`, `session_id`, `source` and `transcript_path`. The fixture emitted one JSON object on stdout. Qualification covers local orchestration; cloud-orchestrated sessions are outside this profile. Desktop bundle 26.1120.50957 was present; the verified lifecycle used the CLI app-server.

<!-- kadai:log:start -->

## Log

### 2026-11-26T23:53:50-03:00

Partial host verification on macOS 27.2.1 ARM64 (26B131), CLI/app-server 0.172.0, installed desktop bundle 26.1120.50957. In a disposable Codex profile, marketplace add and plugin add installed a local probe. App-server plugin/read reported the enabled namespaced skill probe:probe and SessionStart hook. hooks/list reported the startup|resume|compact command as untrusted. Starting an ephemeral thread produced no marker file before the app server was closed. Independently initialized the installed BigBrain stdio MCP server and called query against an isolated database, receiving a successful v1 empty result. These checks establish discovery and read-only MCP. The isolated TUI requests login before the host trust review. Trusted startup/resume, the compact marker reaching the next model request and embedded rendering remain unverified.

### 2026-11-27T02:00:59-03:00

After user authorization, the real CLI delivered startup, resume and compact and returned the marker on the immediate continuation. Simple-code review limited supported claims to exercised surfaces.

<!-- kadai:log:end -->

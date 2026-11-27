---
kadai: task
id: "BB-0041"
title: "Restore context at startup, resume and compaction"
status: "todo"
parent: BB-0034
depends_on: [BB-0039, BB-0040]
---
Wire the bounded context projection to the verified `SessionStart` events. Resolve binding from event cwd, pass the deadline through [Git resolution](../../cmd/bigbrain/project.go) and [daemon calls](../../internal/daemon/client.go), and emit the host's JSON on stdout. Keep prompt and subagent recall off by default.

## Acceptance criteria

- Trusted startup/resume/compact deliver cited context to the next model request. Compact starts a new epoch. Establish resume behavior with the real host. Leave clear/fork gated until their semantics are verified.
- Within an epoch, deduplicate by database, scope/global selection, memory ID and content hash, partitioned by worktree/session/agent. Changed content may be emitted again. Exercise duplicate concurrent invocations and two later compactions so neither is permanently suppressed.
- The handler completes or skips within two seconds under a three-second host timeout, including project discovery. Missing runtime, slow/stopped daemon or malformed input leaves the session usable with valid non-blocking output. Disabled prompt recall makes zero daemon queries.
- Use atomic local state updates and a short scoped lock outside daemon waits. A failure to deliver context can be retried without suppressing all subsequent recall.

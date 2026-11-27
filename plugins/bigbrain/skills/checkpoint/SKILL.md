---
name: checkpoint
description: Prepare a source-linked project handoff after a meaningful unit of work.
---
Select the decision, supporting observation, source reference, unresolved
questions and next steps. Name the originating project. For example, link a
transport decision to the observed daemon behavior and its code location.
Keep open questions in the handoff. Offer the candidate for review before
saving automatic capture. When the user explicitly asks to remember something,
use the authorized project record operation, preserving its payload and key
across retries. Global publication is a separate reviewed operation.

Stage with `bigbrain handoff stage --session SESSION`, passing one JSON object
on stdin. Use the actual host session identifier. For example:

```json
{"producer":"agent handoff","memory":{"type":"decision","text":"Use a Unix socket.","source":"design/transport.md"},"evidence":{"type":"observation","text":"The daemon owns the local socket.","source":"internal/daemon/server.go"},"open_questions":["Which OS versions have been tested?"]}
```

Use `handoff list` in the same session. Edits and dismissals require the draft
ID and the revision last reviewed. Staging preserves the draft immediately.

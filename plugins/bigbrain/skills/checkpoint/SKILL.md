---
name: checkpoint
description: Save requested project or global memories, prepare source-linked handoffs, and suggest reusable project lessons for reviewed global publication.
---

## Explicit requests to remember

Save a memory the user asks to remember in the current project by default. When
they explicitly ask to remember it across projects (for example, “guarda isso
para todos os projetos”), write it directly to the global scope. The request
authorizes that write. Use the user's stated preference or fact and its source.

```sh
bigbrain --json record --global --type TYPE --text "Requested memory" --source "Source" --key KEY
```

Use `--current-project` for a project memory. With the full BigBrain MCP,
`record` accepts `scope: "global"` or `scope: "project:ID"`. When only the
Explorer MCP is available, use the CLI. Use a fresh key for each intentional
write and preserve the database, scope, key and payload on retry. Confirm the
returned memory ID and destination.

## Project handoffs

Select the decision, supporting observation, source reference, unresolved
questions and next steps. Name the originating project. For example, link a
transport decision to the observed daemon behavior and its code location.
Keep open questions in the handoff. Offer the candidate for review before
saving automatic capture.

Stage with `bigbrain handoff stage --automatic --session SESSION`, passing one JSON object
on stdin. Use the actual host session identifier. For example:

```json
{"producer":"agent handoff","memory":{"type":"decision","text":"Use a Unix socket.","source":"design/transport.md"},"evidence":{"type":"observation","text":"The daemon owns the local socket.","source":"internal/daemon/server.go"},"open_questions":["Which OS versions have been tested?"]}
```

Use `handoff list` in the same session. Edits and dismissals require the draft
ID and the revision last reviewed. Staging preserves the draft immediately.
After review, run `bigbrain handoff save --session SESSION --id ID --revision N`.
If acknowledgement is lost, repeat the same command. Confirm each item in the
result before telling the user a handoff was saved.

## Reusable project lessons

When project evidence supports a lesson you would apply in another project,
suggest publishing it. Explain when it applies and show the exact text and
source reference other projects would see. Keep project-specific evidence in
the originating project. Wait for the user's approval of the global text and
source, even if they already approved the project handoff.

After approval, use the existing project proposal ID or create one from the
saved project evidence. Repeat `--evidence` for each supporting memory. Publish
from the originating project with the approved wording and a separate key.

```sh
bigbrain --json consolidate --project PROJECT_ID --evidence MEMORY_ID --key PROPOSAL_KEY
bigbrain --json publish --project PROJECT_ID --proposal PROPOSAL_ID \
  --text "Approved lesson" --source "Approved source" --approve --key PUBLICATION_KEY
```

Use the proposal ID returned by `consolidate` and report the global lesson ID
returned by `publish`.

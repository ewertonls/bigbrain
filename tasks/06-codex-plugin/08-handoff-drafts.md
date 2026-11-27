---
kadai: task
id: "BB-0042"
title: "Stage, edit and dismiss durable handoff drafts"
status: "done"
parent: BB-0034
depends_on: [BB-0038]
---
Add draft operations to the [Go CLI](../../cmd/bigbrain/main.go). Store revisions in private shared helper state, including the originating database/project, worktree/session, producer, selected content and attributed source references. Reuse [domain memory types](../../internal/protocol/protocol.go).

## Acceptance criteria

- After stage returns, restart the process and list the draft with its original scope and sources. Editing an unsubmitted draft creates a new revision. Dismiss affects only the selected draft. Ordinary memory queries exclude staged drafts.
- Enforce a maximum of 50 pending drafts per session and 64 KiB for each complete serialized draft. Reject an oversized or extra draft while retaining earlier work, including uncertain and partially saved operations.
- Serialize concurrent edits under a scoped lock and replace files atomically. Open questions remain a section in the handoff. Staging creates neither a searchable lesson proposal nor a canonical memory.

<!-- kadai:log:start -->

## Log

### 2026-11-27T02:08:09-03:00

Verified 50 pending drafts across separate CLI processes, oversized input rejection, stale-revision conflict, dismissal and no canonical database creation. Simple-code review reused domain record validation and kept optimistic revisions and atomic updates in one mutation path.

<!-- kadai:log:end -->

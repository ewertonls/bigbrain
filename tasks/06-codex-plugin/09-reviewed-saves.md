---
kadai: task
id: "BB-0043"
title: "Save reviewed handoffs with resumable writes"
status: "done"
parent: BB-0034
depends_on: [BB-0039, BB-0041, BB-0042]
---
Extend the [existing handoff writer's](../../integrations/codex/bigbrain-handoff/scripts/record.py) decision, evidence and link sequence using the [daemon client](../../internal/daemon/client.go). Freeze the approved revision, originating database/project and per-operation keys before sending. Persist returned IDs and freeze the concrete link payload before its own write. Update the checkpoint skill to use the workflow.

## Acceptance criteria

- Lose the acknowledgement after a committed record, restart and replay the frozen payload/key. Recover the same IDs and finish the link exactly once. Track which steps succeeded and distinguish partial, uncertain and confirmed saves. A changed revision receives a distinct reviewed request after an uncertain write is resolved.
- Switching viewer scope cannot retarget a save. Only after all steps are acknowledged, atomically advance the latest-reviewed-handoff pointer for the database/project with up to five canonical IDs, worktree and time. A partial save retains the previous pointer.
- `Stop` and `PreCompact` may flush already staged data or retry authorized immutable requests within the two-second budget. Unfinished work remains durable. Repeated Stop events never request continuation. The agent selects semantic content while it is available.

<!-- kadai:log:start -->

## Log

### 2026-11-27T02:09:34-03:00

Verified replay after an already committed first record, stable record and link IDs, rejection of edits after submission and bootstrap from a separate session. The smoke test found that a new DB beneath a symlink changed its scope key at first creation. Fixed the shared resolver to canonicalize its existing prefix, then reran successfully. Simple-code review kept scoped locks outside RPCs and used existing record/link idempotency.

<!-- kadai:log:end -->

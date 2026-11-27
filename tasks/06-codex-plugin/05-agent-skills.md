---
kadai: task
id: "BB-0039"
title: "Package recall, checkpoint and explorer skills"
status: "done"
parent: BB-0034
depends_on: [BB-0037, BB-0038]
---
Adapt the [Codex handoff skill](../../integrations/codex/bigbrain-handoff/SKILL.md) and [integration guide](../../integrations/README.md) into short on-demand workflows. Explain when to recall evidence, stage a checkpoint and open the explorer. Keep command details in references. Update examples as the underlying commands ship.

## Acceptance criteria

- Follow a recall example from a bound project and recover cited evidence using supported commands.
- A checkpoint example selects decisions, evidence, unresolved questions and next steps after a meaningful unit of work. It identifies the source and originating project.
- An explicit request to remember something can use the existing authorized write. Automatic capture stages a draft for review. Publishing approved text across projects remains a separate action.

<!-- kadai:log:start -->

## Log

### 2026-11-27T02:04:11-03:00

Ran the recall example against a disposable bound project and verified a real memory ID and source. Reviewed all three skill entry points with simple-code and kept examples on shipped commands.

<!-- kadai:log:end -->

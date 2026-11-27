---
kadai: task
id: "BB-0039"
title: "Package recall, checkpoint and explorer skills"
status: "todo"
parent: BB-0034
depends_on: [BB-0037, BB-0038]
---
Adapt the [Codex handoff skill](../../integrations/codex/bigbrain-handoff/SKILL.md) and [integration guide](../../integrations/README.md) into short on-demand workflows. Explain when to recall evidence, stage a checkpoint and open the explorer. Keep command details in references. Update examples as the underlying commands ship.

## Acceptance criteria

- Follow a recall example from a bound project and recover cited evidence using supported commands.
- A checkpoint example selects decisions, evidence, unresolved questions and next steps after a meaningful unit of work. It identifies the source and originating project.
- An explicit request to remember something can use the existing authorized write. Automatic capture stages a draft for review. Publishing approved text across projects remains a separate action.

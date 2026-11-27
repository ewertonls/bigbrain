---
kadai: task
id: "BB-0050"
title: "Verify offline installation and continuity across sessions"
status: "todo"
parent: BB-0034
depends_on: [BB-0049]
---
Run the real manual install and handoff flow on each claimed platform. Reuse the [agent acceptance helpers](../../tests/acceptance/agents.go), [integration checks](../../tests/acceptance/integration.go) and one small A/global/B fixture. Record versions, results and limitations in the task log.

## Acceptance criteria

- In session A, stage and review a handoff, interrupt one acknowledgement, then resume. In a fresh session, recover the same evidence and source within the context budget. Verify compaction delivers it again and a draft dismissed before saving never appears.
- Exercise two projects and two worktrees of one project. Check routing, separate session state, explicit global inclusion and the actual retrieval fallback with embeddings unavailable. The local viewer and authorized hooks work offline with the optional companion disabled.
- Verify a clean install, upgrade and disable on macOS ARM64 and on each additional advertised target. Run `make test-integration` for the CLI/daemon flow and `make test` for the release. Attach observed outcomes, including keyboard search, source inspection and reviewed save across the three [reference screens](references/ui-specification.md).

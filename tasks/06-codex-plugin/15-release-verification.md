---
kadai: task
id: "BB-0050"
title: "Verify offline installation and continuity across sessions"
status: "done"
parent: BB-0034
depends_on: [BB-0049]
---
Run the real manual install and handoff flow on each claimed platform. Reuse the [agent acceptance helpers](../../tests/acceptance/agents.go), [integration checks](../../tests/acceptance/integration.go) and one small A/global/B fixture. Record versions, results and limitations in the task log.

## Acceptance criteria

- In session A, stage and review a handoff, interrupt one acknowledgement, then resume. In a fresh session, recover the same evidence and source within the context budget. Verify compaction delivers it again and a draft dismissed before saving never appears.
- Exercise two projects and two worktrees of one project. Check routing, separate session state, explicit global inclusion and the actual retrieval fallback with embeddings unavailable. The local viewer and authorized hooks work offline with the optional companion disabled.
- Verify a clean install, upgrade and disable on macOS ARM64 and on each additional advertised target. Run `make test-integration` for the CLI/daemon flow and `make test` for the release. Attach observed outcomes, including keyboard search, source inspection and reviewed save across the three [reference screens](references/ui-specification.md).

<!-- kadai:log:start -->

## Log

### 2026-11-27T02:32:59-03:00

Verified on macOS ARM64 27.2.1, Codex CLI/app-server 0.172.0 and LadybugDB 0.17.0. A fresh session recovered the previously reviewed handoff. All three installed BigBrain hooks were trusted through the host. Started the session, reconnected and resumed it, compacted it, then requested the release token and source on the next turn. The model returned both without tools. The same session recorded startup, resume, PreCompact, compact and Stop. Its compact receipt identified the saved memory and measured 307 of 8,192 bytes. The offline test drove the installed hook commands with the same event fields. The hosted model test used a network connection.

Verified installation outside the checkout, Git-based upgrade, native replacement, disable (three hooks became zero) and uninstall. Draft and canonical memory survived removal. Tested separate project and worktree bindings, global scope and model_unavailable retrieval. Chromium allowed only loopback traffic and reported no external requests. Used keyboard search, inspected a relationship and source, saved a selected draft, previewed context and paused automation at a width of 320 pixels. Screenshots: [Explorer](images/verified-explorer.png), [Session review](images/verified-review.png), [Automation](images/verified-automation.png).

Simple-code review corrected stable database identity before creation, retained the full recall selection across partial emissions, bounded the complete hook operation and finished reviewed saves after publishing the recovery pointer. A new integration scenario covers acknowledgement loss, replay, dismissal and recovery from a different session. A fresh sandboxed Codex shell also recovered the source with a permission profile allowing the fixture database socket. make test, all five make test-integration scenarios, go vet and the CLI race tests passed.


<!-- kadai:log:end -->

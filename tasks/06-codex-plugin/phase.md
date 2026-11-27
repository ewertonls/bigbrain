---
kadai: task
id: "BB-0034"
title: "06 · Codex plugin"
status: "done"
depends_on: [BB-0006, BB-0050]
---
Deliver a local Codex plugin that can resume work with source-linked BigBrain memories, review a handoff before saving it, and inspect both workflows in a browser. The existing Go daemon owns LadybugDB and serves CLI, hooks and viewer through the Unix socket. Reuse the [completed Integration work](../05-integration/phase.md) for handoffs, MCP and graph explanations.

## Completion criteria

- **Early CLI milestone (P2).** A developer installs the base plugin, binds a project and receives bounded evidence at startup, resume and after compaction.
- **Local release (P6).** On a clean supported installation, a second session recovers a reviewed handoff. Explorer, Session review and Automation work offline with the embedded companion disabled. Record the tested client, platform and limitations.
- **Optional extension (P5).** The read-only Explorer opens beside a conversation in Codex Desktop. The panel is installed separately from the browser viewer.

## Visual references

Design each screen like a compact interactive explanation. Explorer traces a match back to its source, Session review pairs a proposed decision with its evidence and destination, and Automation accounts for the context emitted. Use the [interaction specification](references/ui-specification.md) and the three selected-state references: [retrieval path](images/explorer-retrieval-path.png), [reviewed handoff](images/review-decision-evidence.png) and [recall budget](images/automation-context-budget.png).

Collection-wide browsing, source freshness, lesson publication UI and relationship feedback are deferred. Manual distribution targets macOS ARM64 first. Add Linux ARM64 to the supported profiles after its clean-install verification.

## Work packages

| Stage | Plan | Kadai task |
| --- | --- | --- |
| P0 | WP-01 | [BB-0035 · Verify Codex plugin discovery, hooks and embedded UI support](01-host-capabilities.md) |
| P0 | WP-02 | [BB-0036 · Verify the native runtime outside the checkout](02-native-install.md) |
| P1 | WP-03 | [BB-0037 · Package the base plugin and repository marketplace](03-base-package.md) |
| P1 | WP-04 | [BB-0038 · Bind plugin sessions to a project and diagnose setup](04-project-binding.md) |
| P1 | WP-05 | [BB-0039 · Package recall, checkpoint and explorer skills](05-agent-skills.md) |
| P2 | WP-06 | [BB-0040 · Return bounded, cited context and a recall receipt](06-bounded-context.md) |
| P2 | WP-07 | [BB-0041 · Restore context at startup, resume and compaction](07-recall-hooks.md) |
| P3 | WP-08 | [BB-0042 · Stage, edit and dismiss durable handoff drafts](08-handoff-drafts.md) |
| P3 | WP-09 | [BB-0043 · Save reviewed handoffs with resumable writes](09-reviewed-saves.md) |
| P4 | WP-10 | [BB-0044 · Serve the bundled viewer through a local Go bridge](10-local-viewer.md) |
| P4 | WP-11 | [BB-0045 · Build the memory Explorer and source inspector](11-memory-explorer.md) |
| P4 | WP-12 | [BB-0046 · Build the Session review screen for handoff drafts](12a-session-review.md) |
| P4 | WP-12 | [BB-0047 · Build the Automation screen for recall and capture](12b-automation.md) |
| P5 | WP-13 | [BB-0048 · Open Explorer beside a Codex Desktop conversation](13-embedded-viewer.md) |
| P6 | WP-14 | [BB-0049 · Document manual installation, upgrade and removal](14-manual-distribution.md) |
| P6 | WP-15 | [BB-0050 · Verify offline installation and continuity across sessions](15-release-verification.md) |

<!-- kadai:log:start -->

## Log

### 2026-11-27T02:33:57-03:00

Completed the local macOS ARM64 release after reviewing each work package with simple-code. The optional embedded adapter remains gated on a verified rendering host. See the [delivery report, commits and validation](references/release-report.md).

<!-- kadai:log:end -->

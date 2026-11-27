---
kadai: task
id: "BB-0049"
title: "Document manual installation, upgrade and removal"
status: "done"
parent: BB-0034
depends_on: [BB-0036, BB-0041, BB-0043, BB-0046, BB-0047]
---
Write a tested setup path for a second developer using the [README](../../README.md), [integration guide](../../integrations/README.md) and the selected [native package](../../Dockerfile). Cover local-checkout and repository marketplace installation, native artifacts and library layout, explicit binding, host trust and local browser fallback.

## Acceptance criteria

- Follow the instructions outside the source checkout with no maintainer-specific path. State the tested client, OS/architecture, executable and library requirements. Explain stale executable-path recovery and when to restart MCP after rebinding.
- Demonstrate an upgrade of package assets and the native binary/library. Distinguish plugin, CLI and daemon versions, shut down the old daemon cleanly and verify compatible reconnection. Canonical memory, settings, drafts and frozen manifests survive.
- Disable/remove the plugin and verify automatic hooks stop. Keep the database and pending state unless the user separately chooses to remove plugin-only data. Manual distribution works with the embedded companion omitted.

<!-- kadai:log:start -->

## Log

### 2026-11-27T02:19:42-03:00

Used a clean profile, installed from an isolated Git marketplace, upgraded to a second commit and package version, stopped and replaced the native runtime, and removed the plugin. Verified a canonical memory and pending draft survived. Simple-code review added a stable private executable.path fallback because ordinary setup shells do not receive the host hook environment. Plugin-specific overrides remain supported.

<!-- kadai:log:end -->

---
kadai: task
id: "BB-0037"
title: "Package the base plugin and repository marketplace"
status: "done"
parent: BB-0034
depends_on: [BB-0035, BB-0036]
---
Make the base plugin discoverable through the local checkout and a repository marketplace using the manifest format verified in P0. Provide a setup skill and a small launcher for the separately installed executable. Candidate locations are `.agents/plugins/marketplace.json` and `plugins/bigbrain/`.

## Acceptance criteria

- Install through both tested routes, discover the setup skill and start the binary with MCP disabled. The existing [MCP integration](../../integrations/README.md) remains separately selectable.
- Resolve a validated absolute binary path from the plugin data directory and invoke it as a quoted executable argument. Exercise a path containing spaces with a minimal PATH.
- Keep package assets, executable/state pointers and mutable shared state in their respective locations. Register hooks as their handlers become available and use the host's trust flow.

<!-- kadai:log:start -->

## Log

### 2026-11-27T02:01:56-03:00

Installed with Codex 0.172.0 from the local checkout and from a tagged Git fixture resolved to a local mirror. The setup skill was packaged and both installed launchers ran db-path with an absolute binary path containing spaces and minimal PATH. Used the installed client manifest format. Simple-code review kept the launcher to an absolute-path check and quoted exec.

<!-- kadai:log:end -->

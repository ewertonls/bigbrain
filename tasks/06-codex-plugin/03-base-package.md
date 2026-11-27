---
kadai: task
id: "BB-0037"
title: "Package the base plugin and repository marketplace"
status: "todo"
parent: BB-0034
depends_on: [BB-0035, BB-0036]
---
Make the base plugin discoverable through the local checkout and a repository marketplace using the manifest format verified in P0. Provide a setup skill and a small launcher for the separately installed executable. Candidate locations are `.agents/plugins/marketplace.json` and `plugins/bigbrain/`.

## Acceptance criteria

- Install through both tested routes, discover the setup skill and start the binary with MCP disabled. The existing [MCP integration](../../integrations/README.md) remains separately selectable.
- Resolve a validated absolute binary path from the plugin data directory and invoke it as a quoted executable argument. Exercise a path containing spaces with a minimal PATH.
- Keep package assets, executable/state pointers and mutable shared state in their respective locations. Register hooks as their handlers become available and use the host's trust flow.

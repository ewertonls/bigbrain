---
kadai: task
id: "BB-0048"
title: "Open the BigBrain Explorer beside a Codex Desktop conversation"
status: "done"
parent: BB-0034
depends_on: [BB-0035, BB-0046, BB-0047]
---
Deliver the existing Explorer as a read-only Conversation panel beside the active chat in Codex Desktop. Register a thread entrypoint and a bundled MCP Apps resource. Keep the CLI plugin and browser viewer available independently.

## Acceptance criteria

- Open **BigBrain Explorer** from the desktop conversation's panel picker. Search the bound project, select a memory and verify its exact source in the panel.
- Pin the workspace and database on the server and revalidate that binding on each read. Reuse the Explorer and its scoped query and history contracts. Send detailed results to the app and keep model-visible output compact.
- Record the desktop build, the selected fixture and source, the simple-code review and checks. A stdio probe or standalone browser alone does not establish desktop acceptance.

[Explorer reference](images/explorer-retrieval-path.png) · [Thread entrypoint specification](https://github.com/openai/mcp-extensions/blob/main/docs/spec.md#thread-entrypoint)

<!-- kadai:log:start -->

## Log

### 2026-11-27T02:16:13-03:00

FALLBACK on the tested local Codex 0.172.0 app-server. A disposable stdio MCP server registered a viewer tool, an app-only acknowledgement tool and ui://probe/view.html (text/html;profile=mcp-app). The real client called the tool and read its HTML resource, but did not advertise the MCP Apps UI extension at initialize. A rendered view, app-only visibility enforcement and a UI-originated acknowledgement were not demonstrated. Simple-code review retained the existing browser frontend and deferred the optional adapter until a rendering host qualifies. This task stays open.

### 2026-11-27T02:49:22-03:00

PASS in the actual Codex Desktop application (com.openai.codex, 26.1120.50957, macOS ARM64). Registered a local read-only MCP server against a dedicated database and workspace. The user opened BigBrain Explorer beside this chat. The panel returned two results for Transport. Selecting the observation “Transport evidence shows one daemon owns the database.” changed the inspector from docs/transport.md#unix-socket to tests/fixtures/daemon-owner.txt. The desktop MCP Apps inspector and the user's own selection both confirmed the latter source.

Simple-code review consolidated the Explorer markup, JavaScript, scope checks and daemon reads across the browser and desktop. An empty workspace argument fails rather than opening the general-purpose MCP server. The acceptance test covers entrypoint discovery, the bundled resource, empty launch arguments, source delivery in app metadata, cross-project rejection and a changed binding. Passed make test, all six make test-integration scenarios, go vet for the changed packages, the macOS package build and the browser regression for Explorer, Session review and Automation.


<!-- kadai:log:end -->

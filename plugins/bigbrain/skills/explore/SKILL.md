---
name: explore
description: Inspect how a BigBrain query reached a memory and its source.
---
When invoked without a search term in Claude Code, direct the user to
`/bigbrain-explorer` for the native Overview, Memories, Graph and Activity
tabs. For an explicit search, use the configured launcher from the bound
workspace and run `bigbrain --json graph --current-project --include-global
--text "SEARCH TERMS"` with the user's actual terms. Inspect the selected match, its source and
returned relationship steps. Use history on a selected ID to inspect related
records. Report the stored direction and any reverse traversal. Scope changes
require an explicit selection.

In Claude Code, open the native panel with `/bigbrain-explorer`. For CLI
operations, invoke `"${CLAUDE_PLUGIN_ROOT}/scripts/bigbrain" --claude`.
If the user requests the browser viewer in Claude, first run
`plugin doctor --start-daemon` through that launcher. After it exits, run
`ui --connect-only --session "${CLAUDE_SESSION_ID}"` through the same launcher.

For interactive search and source inspection, run `bigbrain ui` from the
workspace. Open the printed loopback URL locally. Keep its launch fragment
private. Stop the viewer process when finished.

In Codex Desktop, the optional BigBrain Explorer conversation panel opens
directly on recent memories. Use Graph to inspect stored relationships. Use
Memories for a specific retrieval explanation. Both show scope and source.

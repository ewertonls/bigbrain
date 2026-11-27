---
name: setup
description: Configure the native BigBrain executable for local project memory.
---
Verify the user's installed binary by absolute path with `"$executable" --help`.
For a macOS ARM64 distribution, keep the adjacent bin and lib directories.

Use the private shared state directory from `BIGBRAIN_CODEX_STATE`, otherwise
`${XDG_STATE_HOME:-$HOME/.local/state}/bigbrain/codex`. Create it with mode
0700 and write the verified absolute binary path plus a newline to
`executable.path` with mode 0600. A plugin-specific `PLUGIN_DATA/executable.path`
takes precedence. An optional `PLUGIN_DATA/state.path` points to a shared state
override. Keep pointers outside the versioned plugin cache.

Run the installed launcher's `--json plugin doctor` from the user's workspace.
Ask for a project identifier before initializing a previously unbound workspace.
Review command hooks through the host and use the current host session ID for
session-specific drafts, policy and receipts.

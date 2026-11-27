---
name: setup
description: Configure the native BigBrain executable for local project memory.
---
Locate the user's separately installed BigBrain binary. Verify its absolute
path with `"$executable" --help`. On macOS ARM64 the relocatable distribution
contains adjacent bin and lib directories.

In the plugin's persistent data directory (`PLUGIN_DATA`, or
`CLAUDE_PLUGIN_DATA` in command hooks), create the directory with mode 0700.
Write the verified absolute path followed by a newline to `executable.path`
using umask 077. Keep it outside the versioned plugin cache. Verify by running
the plugin's `scripts/bigbrain --json db-path --current-project` from the user's
workspace. For an unconfigured project, ask for its identifier before running
`project init ID`. Review installed hooks through the host's trust interface.

Run `scripts/bigbrain --json plugin doctor` in the workspace to inspect the
logical scope, physical worktree, daemon and shared state directory. An
optional one-line `state.path` next to `executable.path` selects the same
absolute private state directory for multiple plugin packages. A missing
executable is reported by the launcher, and the native loader identifies a
missing library before the doctor command can start.

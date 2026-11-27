---
kadai: task
id: "BB-0038"
title: "Bind plugin sessions to a project and diagnose setup"
status: "todo"
parent: BB-0034
depends_on: [BB-0037]
---
Reuse the [project resolver](../../cmd/bigbrain/project.go), [CLI discovery commands](../../cmd/bigbrain/main.go) and [daemon target](../../internal/daemon/target.go). Establish canonical database, logical project, physical worktree, session and agent from the caller's workspace. Propagate caller deadlines through Git resolution, which currently has a five-second background timeout.

## Acceptance criteria

- Two worktrees of A share a project ID with independent session state. A separate project B and a process started in the plugin cache cannot accidentally select A. Revalidate when the workspace or database changes. Missing binding produces an actionable setup diagnostic.
- Bound project reads explicitly send the project and `include_global: true` by default. Demonstrate project-only reads and global-only reads with `include_global: false`. Write destinations stay explicit.
- Put shared helper state under `$XDG_STATE_HOME/bigbrain/codex` (fallback `~/.local/state/bigbrain/codex`) or an explicit override, with private directories/files (0700/0600). Both plugins can point to it from their own plugin data directories. Use validated or hashed identifiers for filenames.
- Diagnostics distinguish a missing binary/library, incompatible daemon, unresolved project and unsupported host without exposing stored content.

# Cross-agent handoff

The Codex writer and Claude Code reader use the local BigBrain CLI. Use a database directory accessible to both sessions on the same machine. Put `bigbrain` on both agents' `PATH` or set `BIGBRAIN_BIN` to its absolute path. The writer uses Python 3 with its standard library. The reader uses a POSIX shell.

To install for one project, run from its root and set `integration_dir` to this directory:

```sh
integration_dir=/absolute/path/to/bigbrain/integrations
mkdir -p .agents/skills .claude/skills
cp -R "$integration_dir/codex/bigbrain-handoff" .agents/skills/
cp -R "$integration_dir/claude-code/bigbrain-resume" .claude/skills/
```

1. In Codex, invoke `$bigbrain-handoff` with the database directory, project ID, decision, evidence and source for each record and the link. Codex saves a manifest and returns the IDs. Keep the manifest for retries. [Writer and manifest example](codex/bigbrain-handoff/SKILL.md)
2. In a new Claude Code session, invoke `/bigbrain-resume` with the same database directory, project ID and the three IDs from the writer. The reader verifies the recorded relationship and retrieves both sources. It also accepts a search phrase when the IDs are unavailable. [Reader](claude-code/bigbrain-resume/SKILL.md)

Both clients also run directly through the commands in their skill files for offline acceptance testing. Project skill installation follows the [Codex skill conventions](https://learn.chatgpt.com/docs/build-skills) and [Claude Code skill conventions](https://code.claude.com/docs/en/skills).

## Install the local Codex plugin

Verified profile: Codex CLI 0.172.0, macOS ARM64 27.2.1 and LadybugDB 0.17.0.
Build with `make package-macos-arm64`. Copy the complete `dist/macos-arm64`
directory to the desired location, keeping bin, lib and licenses together.

In a shell, replace the example path with the absolute installed executable:

```sh
executable="/absolute/path/to/BigBrain/bin/bigbrain"
"$executable" --help
state="${XDG_STATE_HOME:-$HOME/.local/state}/bigbrain/codex"
(umask 077 && mkdir -p "$state" && printf '%s\n' "$executable" > "$state/executable.path")
codex plugin marketplace add /absolute/path/to/bigbrain-checkout
codex plugin add bigbrain@bigbrain-local
```

For a Git installation, use `codex plugin marketplace add OWNER/REPO --ref REF`
with a revision containing the plugin catalog, then the same plugin-add command.
The checkout and native runtime can be installed in separate locations. If the
binary moves, update the pointer. A plugin-specific pointer takes precedence
when present. The launcher identifies the selected pointer in its error output.

From the target workspace, initialize an unbound project once with
`"$executable" project init PROJECT_ID`. Run `"$executable" --json plugin doctor`.
Start a local Codex session and review the plugin's SessionStart, PreCompact and
Stop commands through `/hooks`. Configuration, host trust and a running daemon
are independent. An explicit query can start the daemon. Automatic recall skips
a stopped daemon. The desktop hook review is described in the
[Codex hooks guide](https://learn.chatgpt.com/docs/hooks).

In a sandboxed Codex shell, a preview can return `DAEMON_UNAVAILABLE` with
`connect: operation not permitted`. The error identifies the database's Unix
socket. A shell needs permission to connect to that socket even for a read-only
query. On the verified macOS client, a read-only permission profile using the
network proxy and an exact socket allowlist entry recovered the handoff. Use
the socket path reported for your database. [Codex permission profile settings](https://learn.chatgpt.com/docs/config-file/config-reference)

Open `"$executable" ui --session SESSION_ID` using the printed URL in a local
browser. Within Codex the CLI defaults to `CODEX_THREAD_ID` when set. The URL
contains a per-launch secret. Search, review and automation run entirely on
loopback; no Node runtime or MCP companion is required. End the foreground
viewer with Ctrl-C. The optional [desktop Explorer panel](#open-explorer-beside-a-desktop-conversation) uses a separate MCP registration, read-only by default.

To update, finish outstanding work, shut down the old daemon with the old
binary and its original DB selection, then replace the entire native package.
Refresh Git sources with `codex plugin marketplace upgrade bigbrain-local` (or
update a local checkout) and reinstall with `codex plugin add bigbrain@bigbrain-local`.
Check doctor, start a new session, and review changed hooks in the host.
Keep the database directory and helper state across upgrades. Restart any
separately configured MCP process after changing its workspace binding.

Use the client's plugin controls to disable BigBrain, or remove the package
with `codex plugin remove bigbrain@bigbrain-local`. Verify in a new session
that its hooks are absent. Removal preserves the separately installed native
binary, canonical database, pending drafts and frozen retry manifests.

### Open Explorer beside a desktop conversation

The optional MCP server serves the project workspace beside a conversation and compact cards inside the chat. It starts read-only.
Build the native package with `make package-macos-arm64`. From the initialized
BigBrain project, register an absolute executable and workspace:

```sh
codex mcp add bigbrain-explorer -- "$executable" mcp --explorer-workspace "$PWD"
```

For a separate database, insert `--db /absolute/path/to/db` before `mcp`.
The registration pins the workspace even when the desktop client launches MCP
from a cache directory. The project name is visible in the scope selector.

In Codex Desktop, open the side panel for the conversation, open its tab menu
and select **BigBrain Explorer**. The **Overview** opens on recent memories.
Open the folder selector to change scope or global inclusion. Select a recent memory to see its source, or **Explore graph** to follow a stored link. **Pending review** opens drafts for the pinned session. The gear opens settings.
**Memories** also supports manual search. Its retrieval explanation shows the
recorded walk, including reverse traversal. The step details preserve the stored direction and source. **Activity** shows up to
20 recent active records. When a session is pinned and global inclusion is
enabled it also shows the latest emitted receipt and observed hook timestamps.
Counts with a trailing `+` are lower bounds from a bounded page.

### Conversation cards

Ask Codex to “show a BigBrain context preview for Unix sockets” to invoke
`recall_card` with a search term. A preview uses the existing context budget
without updating the receipt. To inspect a real receipt or staged drafts, pin
a session by adding `--explorer-session SESSION_ID` after the workspace path
in the registration. Use the same database, workspace, helper state directory
and session as the CLI. This registration remains pinned even if you change
chats. Use a separate registration or restart with the new session when needed.

- Ask “show the latest BigBrain recall receipt” to invoke `recall_card`
  without search text. It shows only emitted IDs, the exact UTF-8 byte count
  and matching source data. Older receipts can lack a stored retrieval reason.
- Ask “review the staged BigBrain handoff” to invoke `review_card`. A normal
  registration previews drafts. To enable **Edit**, **Dismiss selected** and
  the selection-based **Save** button in the card or panel, explicitly add `--explorer-review`
  alongside the pinned session. Only selected draft revisions go through the
  existing reviewed-write and retry path. CLI hooks do not open a card.
- Run `bigbrain ui --session SESSION_ID` for local review and policy controls,
  or `bigbrain --json context --preview --text "Unix sockets"` for a CLI preview.
  To remove the desktop registration, run `codex mcp remove bigbrain-explorer`.

[Conversation panel entrypoints](https://developers.openai.com/plugins/build/extensions) · [MCP Apps UI resources and tool results](https://developers.openai.com/plugins/build/chatgpt-ui)

---
name: recall
description: Recover project decisions and their cited evidence at the start of work.
---
From the bound workspace, use the installed plugin launcher as `bigbrain`.
In Claude Code, invoke `"${CLAUDE_PLUGIN_ROOT}/scripts/bigbrain" --claude`
for the commands below. Pass `--session "${CLAUDE_SESSION_ID}"` to `context`.
Run `bigbrain --json query --current-project --include-global --text "daemon ownership" --limit 5`,
replacing the search text with the current question. Cite the returned memory
ID and source. Keep scope explicit. Treat matches as potentially stale evidence.
If the search is empty, continue with the current task.

For model context, use `bigbrain --json context --text "daemon ownership"`.
The result includes cited excerpts and byte counts. Use `--preview` to inspect
a selection without recording a receipt, or repeat `--id ID` for known memories.

When the user asks for an inline receipt and the optional MCP companion is
available, use `recall_card` without text for the pinned session. For a
requested preview, pass the search text and label the result as a preview.

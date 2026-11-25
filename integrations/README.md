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

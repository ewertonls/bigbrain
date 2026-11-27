---
name: recall
description: Recover project decisions and their cited evidence at the start of work.
---
From the bound workspace, use the installed plugin launcher as `bigbrain`.
Run `bigbrain --json query --current-project --include-global --text "daemon ownership" --limit 5`,
replacing the search text with the current question. Cite the returned memory
ID and source. Keep scope explicit. Treat matches as potentially stale evidence.
If the search is empty, continue with the current task.

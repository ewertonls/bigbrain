---
name: bigbrain-handoff
description: Record an authorized project decision, its supporting evidence and their provenance in BigBrain so another agent can resume the work.
---

Save the requested writes in a private JSON task record before invoking BigBrain. Use the absolute database directory and the explicit project ID agreed with the next agent. Choose a fresh key for each intentional handoff. Cite the decision, evidence and relationship separately.

```json
{
  "database": "/absolute/path/to/database-directory",
  "project": "orbit",
  "key": "orbit/decision-12/review-1",
  "decision": {"text": "Orbit sends readings in batches.", "source": "Decision 12"},
  "evidence": {"text": "The receiver accepts one request per minute.", "source": "Test 7"},
  "link_source": "Review of Decision 12"
}
```

Run `python3 scripts/record.py /absolute/path/to/handoff.json` with the script path resolved relative to this skill. The script uses the v1 CLI from `PATH` or `BIGBRAIN_BIN`. It records the decision and observation and links them with `justified_by`. Report the three returned IDs and a search term from the decision to the receiving agent.

If the process stops or a reply is lost, rerun the saved file as written. Its key supplies the three write keys with fixed suffixes `/decision`, `/evidence` and `/link`. Already committed steps replay. An `IDEMPOTENCY_CONFLICT` indicates that the saved request differs from an earlier write. Inspect the original request before taking further action.

Each step commits independently. If a step returns `INVALID_REQUEST`, correct the rejected field in the saved manifest, preserve the key and the fields of earlier writes, and rerun. For example, correcting a rejected link source reuses the decision and evidence IDs.

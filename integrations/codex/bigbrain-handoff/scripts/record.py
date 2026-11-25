"""Record a project handoff from a saved JSON manifest using the BigBrain v1 CLI."""

import json
import os
from pathlib import Path
import subprocess
import sys


def field(manifest, name):
    value = manifest[name]
    if not isinstance(value, str) or not value.strip() or "\0" in value:
        raise ValueError(f"{name} must be a nonempty string")
    return value


def record(manifest):
    database = field(manifest, "database")
    if not Path(database).is_absolute():
        raise ValueError("database must be an absolute directory path")
    project = field(manifest, "project")
    key = field(manifest, "key")
    decision = manifest["decision"]
    evidence = manifest["evidence"]
    decision_text, decision_source = field(decision, "text"), field(decision, "source")
    evidence_text, evidence_source = field(evidence, "text"), field(evidence, "source")
    link_source = field(manifest, "link_source")

    def write(command, suffix, *args):
        reply = subprocess.run(
            [os.environ.get("BIGBRAIN_BIN", "bigbrain"), "--json", "--db", database,
             command, "--project", project, "--key", f"{key}/{suffix}", *args],
            stdout=subprocess.PIPE,
        )
        if reply.returncode:
            print(f"handoff: {suffix} write failed", file=sys.stderr)
            sys.stdout.buffer.write(reply.stdout)
            raise SystemExit(reply.returncode)
        envelope = json.loads(reply.stdout)
        if envelope["version"] != 1 or not isinstance(envelope["result"]["id"], str):
            raise ValueError("unexpected BigBrain write response")
        return envelope["result"]

    decision_result = write("record", "decision", "--type", "decision",
                            "--text", decision_text, "--source", decision_source)
    evidence_result = write("record", "evidence", "--type", "observation",
                            "--text", evidence_text, "--source", evidence_source)
    link_result = write("link", "link", "--type", "justified_by", "--weight", "1",
                        "--from", decision_result["id"], "--to", evidence_result["id"],
                        "--source", link_source)
    return {"decision": decision_result, "evidence": evidence_result, "relationship": link_result}


if __name__ == "__main__":
    if len(sys.argv) != 2:
        sys.exit("usage: python3 record.py HANDOFF.json")
    try:
        with open(sys.argv[1], encoding="utf-8") as source:
            print(json.dumps(record(json.load(source))))
    except (KeyError, OSError, TypeError, ValueError) as exc:
        sys.exit(f"handoff: {exc}")

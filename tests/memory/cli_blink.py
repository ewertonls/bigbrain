"""Exercise the compiled CLI, its daemon and an offline Blink directory on Linux or macOS."""
import errno
import json
import os
from pathlib import Path
import pty
import shutil
import subprocess
import sys
import tempfile


def main(binary, bundle):
    with tempfile.TemporaryDirectory(prefix="bigbrain-blink-") as tmp:
        root = Path(tmp)
        db = root / "database"
        model = root / "model"
        shutil.copytree(bundle, model)
        subprocess.run(["git", "init", "-q"], cwd=root, check=True)
        env = dict(os.environ, BIGBRAIN_EMBEDDINGS=str(model), BIGBRAIN_VECTOR_INDEX="")

        def run(*args, terminal=False):
            cmd = [binary, "--db", str(db), *args]
            if not terminal:
                proc = subprocess.run(cmd, cwd=root, env=env, capture_output=True, check=True)
                return proc.stdout, proc.stderr
            master, slave = pty.openpty()
            try:
                with os.fdopen(slave, "wb") as stderr:
                    proc = subprocess.run(cmd, cwd=root, env=env, stdout=subprocess.PIPE, stderr=stderr, timeout=60)
                output = bytearray()
                try:
                    while block := os.read(master, 4096):
                        output.extend(block)
                except OSError as e:
                    if e.errno != errno.EIO:
                        raise
                if proc.returncode:
                    raise RuntimeError((proc.stdout, bytes(output)))
                return proc.stdout, bytes(output)
            finally:
                os.close(master)

        search = ["--include-global", "--mode", "hybrid", "--text", "queue"]
        query = ["query", "--current-project", *search]
        try:
            run("project", "init", "blink-smoke")
            run("record", "--current-project", "--key", "first", "--text", "queue", "--source", "Queue design")
            raw, bar = run(*query, terminal=True)
            result = json.loads(raw)
            first = result["matches"][0]
            assert first["source"] == "Queue design" and first["reason"]["cosine"] > .999
            assert b"recalculation project:blink-smoke [" in bar and b"1/1" in bar, bar

            run("record", "--global", "--key", "shared", "--text", "queue recovery", "--source", "Global runbook")
            run("record", "--project", "neighbor", "--key", "private", "--text", "queue", "--source", "Neighbor only")
            raw, bar = run("--json", *query, terminal=True)
            result = json.loads(raw)["result"]
            assert b"recalculation global [" in bar and bar.count(b"1/1") == 1, bar
            assert {m["source"] for m in result["matches"]} == {"Queue design", "Global runbook"}
            raw, bar = run("--json", "query", "--project", "neighbor", *search, terminal=True)
            assert b"recalculation project:neighbor [" in bar and bar.count(b"1/1") == 1, bar
            assert b"recalculation global [" not in bar, bar
            assert {m["source"] for m in json.loads(raw)["result"]["matches"]} == {"Neighbor only", "Global runbook"}
            raw, bar = run("--json", *query, terminal=True)
            assert json.loads(raw)["result"] == result and not bar, bar

            run("shutdown")
            raw, bar = run("--json", *query, terminal=True)
            assert json.loads(raw)["result"] == result and not bar, bar

            run("record", "--current-project", "--key", "second", "--text", "queue recovery", "--source", "Recovery drill")
            raw, bar = run("--json", *query, terminal=True)
            assert json.loads(raw)["result"]["count"] >= 1 and b"1/1" in bar, bar

            # Update the tokenizer file in place at the bundle path. Even a
            # semantics-preserving byte change selects a fresh fingerprint.
            rules = model / "tokenizer.json"
            replacement = model / "replacement"
            replacement.write_bytes(rules.read_bytes() + b"\n")
            replacement.replace(rules)
            raw, bar = run("--json", *query, terminal=True)
            assert "fallback" not in json.loads(raw)["result"]
            for phase in (b"invalidation", b"recalculation"):
                assert phase + b" project:blink-smoke [" in bar
            assert bar.count(b"2/2") == 2, bar

            rules.write_text("{}")
            raw, diagnostic = run("--json", *query)
            assert json.loads(raw)["result"]["fallback"] == "model_unavailable"
            assert b"daemon.log" in diagnostic and "tokenizer.json" in (db / "daemon.log").read_text()
            rules.unlink()
            raw, _ = run("--json", *query)
            assert json.loads(raw)["result"]["fallback"] == "model_unavailable"
        finally:
            run("shutdown")

        db = root / "fresh-database"
        env["BIGBRAIN_EMBEDDINGS"] = str(root / "missing-bundle")
        try:
            run("record", "--current-project", "--key", "fallback", "--text", "queue", "--source", "Fallback control")
            raw, diagnostic = run("--json", *query)
            fallback = json.loads(raw)["result"]
            assert fallback.pop("fallback") == "model_unavailable" and b"daemon.log" in diagnostic
            assert fallback["matches"][0]["source"] == "Fallback control"
            control, _ = run("--json", "query", "--current-project", "--include-global", "--mode", "associative", "--text", "queue")
            assert fallback == json.loads(control)["result"]
        finally:
            run("shutdown")
    print("CLI passed: Blink cosine, TTY progress, JSON, project isolation, shared global cache, restart, single write, tokenizer replacement and fresh missing-model fallback")


if __name__ == "__main__":
    main(str(Path(sys.argv[1]).resolve()), Path(sys.argv[2]).resolve())

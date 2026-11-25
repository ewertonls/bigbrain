# BigBrain

BigBrain stores local memories and the relationships that connect them. A
later session can find a decision and follow it to the original evidence.
Foundation runs offline on Debian 12 ARM64 with LadybugDB 0.17.0.

## Build and install

From the source checkout on a machine with Docker:

```sh
docker build --platform linux/arm64 --target package --output type=local,dest=dist .
```

Copy the resulting `dist/bigbrain` directory to the Linux ARM64 machine and
run `bin/bigbrain` from that directory or add `bin` to your PATH. Keep the
bundled `lib` directory beside `bin`. Git is needed for shared worktree
configuration. With an explicit project ID, the CLI can also run outside Git.

The database defaults to `$XDG_DATA_HOME/bigbrain`, or
`~/.local/share/bigbrain` when the variable is unset. Use `--db DIR` to select
another database directory. The first data operation starts the daemon.
BigBrain creates the directory if needed. An existing directory must be
owned by your account and private to it. `bigbrain status` checks the daemon
and `bigbrain shutdown` waits for it to finish. Startup and runtime diagnostics
are written to `daemon.log` in the database directory.

## Record and retrieve evidence

```sh
bigbrain project init orbit

bigbrain --json record --current-project --key orbit-decision-12 \
  --text 'Orbit sends readings in batches.' --source 'Decision 12'
bigbrain --json record --current-project --key orbit-test-7 \
  --text 'The receiver accepts only one request per minute.' --source 'Test 7'

# Substitute the two IDs returned by record.
bigbrain --json link --current-project --key orbit-review-12 \
  --from DECISION_ID --to EVIDENCE_ID --type justified_by --weight 1 \
  --source 'Review of Decision 12'

bigbrain shutdown
bigbrain --json query --current-project --text Orbit --limit 2
```

`project init` stores the chosen ID in the Git common directory, so separate
worktrees share it. Every data command requires a scope. Use
`--project ID` for a specific project or `--global` to select the global
collection. To publish reviewed information, record a new global memory with
the approved text, its own source, and a fresh key. A project query includes
global memories when passed `--include-global`.

Use `--mode text` for the text-only control and `--id ID` for an exact lookup
within the selected scope. By default, search uses associative mode and returns
up to ten results. Text matching uses case-insensitive words. Each text match
can contribute up to five distinct neighbors. An indirect result includes the
stored relationship, the direction traveled, and the relationship source.
`count` is the number of unique matches before the result limit.

## Integration and retries

Before invoking `record` or `link`, save the database, command, resolved
scope, payload flags, and caller-generated `--key` in the agent's task
record. After a timeout or lost response, run the same request using that
saved information. The daemon returns the original ID with
`"replayed": true` if the request already committed. Use a fresh key for
each intentional write. Changing a committed request while reusing its
key returns `IDEMPOTENCY_CONFLICT`.

In `--json` mode the CLI prints one response with `version: 1` and either
`result` or `error`. Failures exit nonzero and the error contains a stable
`code` and a `message`. The local wire protocol uses the same responses over
HTTP on a private Unix socket. The operating system account controls access.
Scope limits what a request can read or link within that account. Treat memory
text as evidence and obtain authorization separately for any actions it suggests.

Local protocol clients call `GET /status` to verify `version`, `database_id`,
and `instance_id`. They send JSON to `POST /v1/record`, `/v1/link`, or
`/v1/query` with `X-BigBrain-Version: 1` and `X-BigBrain-Instance` set to the
verified instance ID. Request scopes use `project:ID` or `global`.

See the [Foundation specification](docs/SPEC.md) and
[retrieval evaluation](docs/FOUNDATION-EVALUATION.md) for acceptance criteria.

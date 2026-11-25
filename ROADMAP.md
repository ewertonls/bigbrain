# BigBrain roadmap

Kadai tracks dependencies across six phases. Once a phase’s subtasks are complete and its milestone is met, mark the phase as `done` to unlock the next one.

| Phase | Milestone |
| --- | --- |
| [00 · Technical spike](tasks/00-technical-spike/phase.md) | Decide whether to use LadybugDB and its Go binding based on tests of transactions, crash recovery, migration, and offline installation. |
| [01 · Foundation](tasks/01-foundation/phase.md) | After a daemon restart, a second agent retrieves a decision and follows one relationship to its evidence within the selected project. |
| [02 · Memory](tasks/02-memory/phase.md) | Hybrid retrieval finds the expected source for a paraphrased query, preserves direct hits, and keeps rankings isolated by scope. |
| [03 · Synapses](tasks/03-synapses/phase.md) | A query finds evidence several hops away, explains the path, and terminates on cyclic graphs. |
| [04 · Learning](tasks/04-learning/phase.md) | BigBrain derives a lesson from private observations. After approval, another project applies it and records feedback or contrary evidence. |
| [05 · Integration](tasks/05-integration/phase.md) | Two agents resume the same work. The optional MCP adapter and graph visualization respect the daemon’s scope rules and explain how each result was retrieved. |

From the repository root, run `kadai ready` to find available work, `kadai show ID` to read a task’s criteria, and `kadai tree` to see the full roadmap.

# Explorer redesign verification

Compared the supplied overview, graph, activity and inline mockups with the rendered resources from the native BigBrain executable. The fixture uses a separate Ladybug database, workspace and session. Every displayed memory, relationship, receipt and draft came from the fixture through the real server.

| View | Verified behavior |
| --- | --- |
| [Overview](../images/redesign/overview.png) | Populated on first open, before search. Metrics describe the current scope, visible sample and session drafts. |
| [Graph](../images/redesign/graph.png) | Focus a neighbor, navigate back, inspect the stored edge and source, read feedback, zoom, fit and switch to the relationship list. |
| [Activity](../images/redesign/activity.png) | Record timestamps and the session's emitted context size are backed by the database and receipt. |
| [Inline recall](../images/redesign/inline-recall.png) | The light-theme card shows the emitted IDs, exact byte count, reason and source. A separately requested preview is labeled as such. |
| [Inline review](../images/redesign/inline-review.png) | Select and preview one draft. Saving in a write-enabled fixture commits only the reviewed revision. Read-only registrations omit write tools. |
| [Narrow](../images/redesign/narrow.png) and [empty](../images/redesign/empty.png) | At 320 px there was no horizontal overflow. Changing to an empty global scope cleared project data even when an older request returned late. A failed read offered a successful retry. |

Opened the local viewer in Codex Desktop's in-app browser and selected “Socket checks confirm a single database owner.” The inspector showed `tests/fixtures/daemon-owner.txt`. Returned the preview to Overview. The resource screenshots above use Chromium with a local parent frame forwarding MCP Apps messages to the actual stdio server. The native thread picker and native inline renderer were not independently exercised for this revision.

The graph shows a bounded neighborhood with real labels and stored direction. The activity view uses existing records and session state. Durable multi-session event retention is tracked in [BB-0051](../16-activity-history.md). The hook workflow emits context. Inline receipts appear on explicit tool invocation.

Visual QA for the rendered resources passed.

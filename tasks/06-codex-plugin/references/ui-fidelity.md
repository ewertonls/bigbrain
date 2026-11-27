# ChatGPT visual references and verified renders

The six original screenshots supplied in the fidelity handoff are the visual target for BB-0048. The first two show one continuous Overview. The plugin uses system typography, a neutral surface, outline icons and host theme variables when provided. Node colors encode memory type. The scope selector and pending review count are interactive.

| Surface | Original reference | Render from the packaged plugin |
| --- | --- | --- |
| Overview | [Top](canonical/01-overview-top.png) · [scrolled](canonical/02-overview-bottom.png) | [Top](../images/fidelity/overview.png) · [scrolled](../images/fidelity/overview-bottom.png) · [full surface](../images/fidelity/workspace.png) |
| Recall | [Context recalled](canonical/03-inline-recall.png) | [Emitted receipt](../images/fidelity/inline-recall.png) |
| Handoff | [New knowledge](canonical/04-inline-review.png) | [Selected draft and destination](../images/fidelity/inline-review.png) |
| Graph | [Two graph modes](canonical/05-graph-modes.png) | [Stored neighborhood](../images/fidelity/graph.png) · [actual retrieval traversal](../images/fidelity/retrieval.png) |
| Activity | [Timeline](canonical/06-activity.png) | [Recorded project and session activity](../images/fidelity/activity.png) |

The preview combines at most three recent neighborhoods and displays the number of unique links in that sample. Graph density and labels follow the actual data. Inline recall shows measured UTF-8 bytes and lets the reader expand provenance and receipt details. Historical receipts retain the included memory IDs and reasons where available. They do not contain a complete traversal. Search results with recorded steps have their own traversal diagram. Handoff drafts start unselected and saves use the explicitly selected revisions.

## Verification

- Compared dark renders with the references at a comparable effective width. Corrected the layout to fit graph coordinates after positioning the nodes and to avoid overlapping labels.
- Captured a [320 px panel](../images/fidelity/narrow.png), [light theme](../images/fidelity/light.png), [empty scope](../images/fidelity/empty.png) and [read failure](../images/fidelity/error.png). Exercised keyboard navigation, theme updates, scope switching with a late response, error recovery, graph selection, source inspection, long draft provenance and selective saving.
- In Codex Desktop's in-app browser, selecting the observation about socket ownership showed the source `tests/fixtures/daemon-owner.txt`. The preview uses a disposable database.
- Captured the packaged panel and inline resources in Chromium through MCP Apps messages forwarded to the real stdio server. Rendering inside the native chat transcript and native panel picker was not independently verified in this revision. The existing [access guide](../../../integrations/README.md#open-explorer-beside-a-desktop-conversation) covers both.
- The [previous verified renders](design-qa.md) remain available for comparison. The durable cross-session activity history remains tracked in [BB-0051](../16-activity-history.md).

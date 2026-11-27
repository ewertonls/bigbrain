---
name: explore
description: Inspect how a BigBrain query reached a memory and its source.
---
Use the configured launcher from the bound workspace. Run
`bigbrain --json graph --current-project --include-global --text "daemon ownership"`
with the user's search terms. Inspect the selected match, its source and
returned relationship steps. Use history on a selected ID to inspect related
records. Report the stored direction and any reverse traversal. Scope changes
require an explicit selection.

For interactive search and source inspection, run `bigbrain ui` from the
workspace. Open the printed loopback URL locally. Keep its launch fragment
private. Stop the viewer process when finished.

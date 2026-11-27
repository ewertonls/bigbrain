---
kadai: task
id: "BB-0044"
title: "Serve the bundled viewer through a local Go bridge"
status: "done"
parent: BB-0034
depends_on: [BB-0038]
---
Add an explicit viewer command to the [Go CLI](../../cmd/bigbrain/main.go). Embed locally built frontend assets in the binary through the [Makefile](../../Makefile). Start an ephemeral loopback listener and forward allowlisted query/history calls through the [daemon client](../../internal/daemon/client.go). Keep the existing static graph command available.

## Acceptance criteria

- Open the viewer offline on a machine with the native runtime and no Node installation. Closing/restarting it or encountering a port conflict leaves the shared daemon running. Show actionable connection/version errors.
- Authenticate every private API read/write using a per-launch secret, enforce the expected Host, validate supplied Origins exactly and define which routes accept an absent Origin. Verify the allowed case and rejection of missing/invalid credentials and foreign origins. Keep the secret out of logs and persistent browser storage.
- Restrict request size, operations and scope. The initial API exposes bounded reads. Later reviewed writes use dedicated routes. The daemon keeps its Unix socket and sole database ownership.

<!-- kadai:log:start -->

## Log

### 2026-11-27T02:10:59-03:00

Embedded offline assets directly with go:embed. Verified expected host, missing/bad launch token, cross-origin access and missing-Origin policy with handler tests. Each viewer gets its own loopback listener and forwards bounded domain calls to the existing daemon. Simple-code review avoided a Node build or general-purpose operation proxy.

<!-- kadai:log:end -->

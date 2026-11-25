---
kadai: task
id: "BB-0001"
title: "00 · Technical spike"
status: "done"
depends_on: [BB-0009, BB-0010, BB-0011]
---
Determine whether LadybugDB and its Go binding can support the local prototype on the first Unix target.

## Completion criteria

Record the decision, versions tested, measurements, crash test results, and evidence from offline installation and migration. If a blocker rules out this combination, document and validate an alternative before starting Foundation.

Complete the subtasks and mark this milestone as `done`.

[Project reference](../../docs/SPEC.md#initial-validation)

<!-- kadai:log:start -->

## Log

### 2026-11-24T21:44:59-03:00

Started the Linux ARM64 LadybugDB technical spike. The selected target is a clean, network-disabled Linux ARM64 container. The spike will qualify environment setup, transactions, crash recovery, version migration, and compiler-free offline distribution.

### 2026-11-24T21:59:11-03:00

Risk discovered in BB-0008: go-ladybug v0.17.0/liblbug v0.17.0 crashes during simultaneous prepared writes from two connections. The spike will verify the supported single-writer queue design described by the Foundation specification and record concurrent binding use as an explicit implementation constraint.

### 2026-11-24T22:09:07-03:00

Decision: proceed with LadybugDB v0.17.0 and go-ladybug v0.17.0 for Foundation on Linux ARM64. All five spike subtasks passed when writes use the daemon-style single-writer queue. The package must include liblbug.so.0.17.0; CLI and daemon modes may share one Go executable, but the tested distribution is not a single static binary. Simultaneous prepared writes through separate Go connections reproducibly SIGSEGV and are prohibited. The complete evidence, measurements, migration result, package contents, and reproduction commands are documented in spikes/ladybug/REPORT.md and README.md.

<!-- kadai:log:end -->

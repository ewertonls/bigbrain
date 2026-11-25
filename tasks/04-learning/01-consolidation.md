---
kadai: task
id: "BB-0027"
title: "Consolidate observations and propose local lessons"
status: "done"
parent: BB-0005
depends_on: [BB-0004]
---
Implement an explicit write operation that groups experiences within one scope and produces a synthesis or proposed lesson. Record when it applies, the rationale, the sources, and who or what produced it. Accept external proposals. If a replaceable inference provider is used, give it access only to evidence visible in that scope.

## Acceptance criteria

Given observations from A, BigBrain produces a lesson that links back to the original experiences. Those experiences remain queryable, and the proposal stays in A until approved.

[Project reference](../../docs/ARCHITECTURE.md#learning-and-the-memory-lifecycle)

<!-- kadai:log:start -->

## Log

### 2026-11-25T14:13:43-03:00

Added project-scoped lesson proposals from explicit evidence, with offline extractive synthesis or externally supplied text and attribution. The proposal, its evidence links and its idempotency receipt are committed together. Verified the v6 migration, project isolation, rollback, deletion and replay through store tests and CLI/daemon checks on macOS ARM64. The full test suite, build and go vet passed.

<!-- kadai:log:end -->

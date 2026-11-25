---
kadai: task
id: "BB-0011"
title: "Test the package on a clean, offline machine"
status: "done"
parent: BB-0001
depends_on: [BB-0007]
---
Package the prototype and its native dependencies for the selected platform. Assess whether the CLI and daemon can ship in one executable.

## Acceptance criteria

Install the package on a clean machine with no compiler and the network disabled. Write the test data, reopen the database, and retrieve the evidence in one hop. Record which files the package ships and any manual steps.

[Project reference](../../docs/SPEC.md#initial-validation)

<!-- kadai:log:start -->

## Log

### 2026-11-24T22:07:49-03:00

Passed on a clean Debian 12 ARM64 runtime with --network none and no Go toolchain or C compiler. The 9.7 MiB archive contains the Go executable, PACKAGE.md, both MIT license files, and liblbug.so.0.17.0 with symlinks. SHA-256: 4e16595da5675c4ddfd5dedbf68c8325b7c6f065de64ba001764e2051c74fc02. After extraction, it created the schema, stored D1/E1/E2 and their relationship, reopened the database, and returned the expected forward and reverse paths. No manual installation step beyond extraction and LD_LIBRARY_PATH is required.

<!-- kadai:log:end -->

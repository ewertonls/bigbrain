---
kadai: task
id: "BB-0007"
title: "Set up a reproducible LadybugDB environment with the Go binding"
status: "done"
parent: BB-0001
---
Choose a Unix OS and CPU architecture, pin the engine and binding versions, and prepare a disposable database with memories, sources, and directed relationships.

## Acceptance criteria

The Go example opens the database, writes a relationship, and follows it in one hop. Record the commands, versions, native libraries, and test data needed to repeat the experiment.

[Project reference](../../docs/SPEC.md#initial-validation)

<!-- kadai:log:start -->

## Log

### 2026-11-24T21:58:20-03:00

Passed on Linux ARM64. Built go-ladybug v0.17.0 against the official liblbug v0.17.0 ARM64 archive (SHA-256 a486000f7305cb601e3074eb06026e9eddcf3d7517df7bbc913d2d36e3108121). Docker image sha256:a7b59d65a3850c0aa878ee8194edd5107e298629816c027163cbcbad9d46efea opened the database, stored D1/E1/E2 and D1 -> justified_by -> E1, reopened it, and returned the expected forward and reverse paths with both provenance sources. Runtime dependencies are the bundled liblbug.so.0 plus glibc, libdl, libpthread, libm, and libgcc_s.

<!-- kadai:log:end -->

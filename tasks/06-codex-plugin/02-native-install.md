---
kadai: task
id: "BB-0036"
title: "Verify the native runtime outside the checkout"
status: "done"
parent: BB-0034
---
Establish how the plugin launcher will run a separately installed BigBrain executable and its matching LadybugDB library. The development build embeds the checkout's library path. Start with macOS ARM64 and record whether the tested install is relocatable or requires a documented local build.

## Acceptance criteria

- Launch with an absolute executable path from an unrelated directory, with spaces in the installation path and a minimal desktop-like PATH. Show the binary can load the library and reach a local daemon.
- Record architecture, minimum verified OS, native library lookup, executable permissions, required licenses and the install layout. A relocatable profile must work when the build checkout is unavailable.
- Limit Linux ARM64 support claims to a verified clean installation. Setup owns dependency installation so an automatic hook can skip a missing runtime immediately.

Start with the [Makefile](../../Makefile), [native download script](../../scripts/download-ladybug.sh) and [Docker package](../../Dockerfile).

<!-- kadai:log:start -->

## Log

### 2026-11-26T23:52:18-03:00

Verified the macOS ARM64 package on macOS 27.2.1, build 26B131. Copied it to a path containing spaces and launched it from outside the checkout with env -i PATH=/usr/bin:/bin and an isolated --db. A text query succeeded, status reported a live daemon, and shutdown completed. otool reported only @executable_path/../lib for the executable rpath. The package includes the LadybugDB 0.17.0 runtime and both license files. Simple-code review reused the existing setup and soname instead of adding an installer or post-link patch. Linux plugin verification remains separate.

<!-- kadai:log:end -->

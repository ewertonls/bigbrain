# BigBrain LadybugDB spike package

- Target: Linux ARM64 with glibc
- Go binding: `github.com/LadybugDB/go-ladybug v0.17.0`
- Native engine: LadybugDB `v0.17.0`
- Native archive SHA-256: `a486000f7305cb601e3074eb06026e9eddcf3d7517df7bbc913d2d36e3108121`
- Bundled runtime library: `lib/liblbug.so.0.17.0`
- System runtime libraries: glibc, `libdl`, `libpthread`, `libm`, and `libgcc_s`

Set `LD_LIBRARY_PATH` to the package's `lib` directory, then invoke
`bin/ladybug-spike`. The package contains no compiler, source tree, or network
client dependency.

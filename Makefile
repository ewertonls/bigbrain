ladybug_dir := $(CURDIR)/lib-ladybug
# The checkout rpath also works for Go test binaries in temporary directories.
go_flags := -tags system_ladybug -ldflags "-r '$(ladybug_dir)'"

export CGO_ENABLED := 1
export CGO_CFLAGS := "-I$(ladybug_dir)"
# The system_ladybug binding supplies -llbug.
export CGO_LDFLAGS := "-L$(ladybug_dir)"

.PHONY: setup build run test test-learning test-integration evaluate-memory clean

build: setup
	go build $(go_flags) -o build/bigbrain ./cmd/bigbrain

.PHONY: package-macos-arm64
package-macos-arm64: setup
	@test "$$(uname -sm)" = "Darwin arm64"
	mkdir -p dist/macos-arm64/bin dist/macos-arm64/lib dist/macos-arm64/licenses
	go build -tags system_ladybug -trimpath -ldflags '-r=@executable_path/../lib' -o dist/macos-arm64/bin/bigbrain ./cmd/bigbrain
	cp -L "$(ladybug_dir)/liblbug.0.dylib" dist/macos-arm64/lib/
	cp spikes/ladybug/LICENSES/*.txt dist/macos-arm64/licenses/

setup:
	@./scripts/download-ladybug.sh

run: build
	./build/bigbrain $(ARGS)

test: setup
	go test $(go_flags) ./...

test-learning: build
	go run $(go_flags) ./tests/acceptance --learning "$(CURDIR)/build/bigbrain"

test-integration: build
	go run $(go_flags) ./tests/acceptance --integration "$(CURDIR)/build/bigbrain"

evaluate-memory: build
	go run $(go_flags) ./tests/acceptance --memory "$(CURDIR)/build/bigbrain" tests/memory/fixture.json
	go run $(go_flags) ./tests/acceptance --memory "$(CURDIR)/build/bigbrain" tests/memory/held-out.json
	@model=$$(realpath "$$BIGBRAIN_EMBEDDINGS") && \
	BIGBRAIN_EMBEDDINGS="$$model" go test $(go_flags) -run '^$$' -bench '^BenchmarkMemoryIndex$$' -benchmem ./internal/store

clean:
	$(RM) build/bigbrain

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
	@model=$$(realpath "$$BIGBRAIN_EMBEDDINGS") && \
	BIGBRAIN_EMBEDDINGS="$$model" go test $(go_flags) -run '^$$' -bench '^BenchmarkMemoryIndex$$' -benchmem ./internal/store

clean:
	$(RM) build/bigbrain

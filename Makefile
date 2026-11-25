ladybug_dir := $(CURDIR)/lib-ladybug
# The checkout rpath also works for Go test binaries in temporary directories.
go_flags := -tags system_ladybug -ldflags "-r '$(ladybug_dir)'"

export CGO_ENABLED := 1
export CGO_CFLAGS := "-I$(ladybug_dir)"
# The system_ladybug binding supplies -llbug.
export CGO_LDFLAGS := "-L$(ladybug_dir)"

.PHONY: setup build run test clean

build: setup
	go build $(go_flags) -o build/bigbrain ./cmd/bigbrain

setup:
	@./scripts/download-ladybug.sh

run: build
	./build/bigbrain $(ARGS)

test: setup
	go test $(go_flags) ./...

clean:
	$(RM) build/bigbrain

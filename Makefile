VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS = -ldflags "-X main.version=$(VERSION) -X 'main.buildTime=$(BUILD_TIME)'"

.PHONY: build install clean test check lint fmt dist snapshot

build: check
	go build $(LDFLAGS) -o bin/tash ./cmd/tash

install: build
	mkdir -p ~/.local/bin
	cp bin/tash ~/.local/bin/
	@echo "Run 'tash init' to generate your profile"

clean:
	rm -rf bin/

test:
	go test ./...

check: lint fmt

lint:
	golangci-lint run ./...

fmt:
	golangci-lint fmt ./... --diff

dist:
	goreleaser release --clean

snapshot:
	goreleaser release --clean --snapshot

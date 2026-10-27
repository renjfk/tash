VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo "dev")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS = -ldflags "-X main.version=$(VERSION) -X 'main.buildTime=$(BUILD_TIME)'"
E2E_API_KEY_ENV ?= E2E_API_KEY

.PHONY: build install clean test check lint fmt dist snapshot e2e e2e-build e2e-docker

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

# End-to-end suite against a real OpenAI-compatible endpoint.
# Requires E2E_ENDPOINT, E2E_MODEL, and the API key in $E2E_API_KEY_ENV
# (default E2E_API_KEY), plus `tui-test` for interactive scenarios.
e2e: build
	./e2e/run.sh $(ARGS)

e2e-build:
	docker build -f e2e/Dockerfile -t tash-e2e .

e2e-docker: e2e-build
	docker run --rm \
		-e E2E_API_KEY_ENV \
		-e E2E_ENDPOINT \
		-e E2E_MODEL \
		-e E2E_MAX_TOKENS_PARAM \
		-e E2E_REASONING_EFFORT \
		-e "$(E2E_API_KEY_ENV)" \
		tash-e2e $(ARGS)

dist:
	goreleaser release --clean

snapshot:
	goreleaser release --clean --snapshot

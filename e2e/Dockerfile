# Reproducible environment for the tash e2e suite: Go toolchain to build tash,
# fish + zellij to exercise the shell integration, and tui-test to drive the
# interactive sessions. The API key is supplied at runtime, not baked in.
#
#   docker build -f e2e/Dockerfile -t tash-e2e .
#   docker run --rm -e E2E_ENDPOINT -e E2E_MODEL -e E2E_API_KEY tash-e2e
#   docker run --rm -e E2E_ENDPOINT -e E2E_MODEL -e E2E_API_KEY tash-e2e 02 10   # subset

FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/tash ./cmd/tash

FROM debian:bookworm-slim

ARG TARGETARCH=amd64
RUN apt-get update \
 && apt-get install -y --no-install-recommends fish curl ca-certificates jq \
 && rm -rf /var/lib/apt/lists/*

# Zellij (optional for the screen-capture scenario).
RUN set -eux; \
    case "${TARGETARCH}" in \
        amd64) zarch=x86_64 ;; \
        arm64) zarch=aarch64 ;; \
        *) zarch=x86_64 ;; \
    esac; \
    curl -fsSL "https://github.com/zellij-org/zellij/releases/latest/download/zellij-${zarch}-unknown-linux-musl.tar.gz" -o /tmp/zellij.tgz; \
    tar -xzf /tmp/zellij.tgz -C /usr/local/bin zellij; \
    rm /tmp/zellij.tgz

# tui-test CLI.
RUN curl --proto '=https' --tlsv1.2 -LsSf https://raw.githubusercontent.com/microsoft/tui-test/main/install/install.sh | sh
ENV PATH="/root/.local/bin:/root/.cargo/bin:${PATH}"

COPY --from=build /out/tash /usr/local/bin/tash
COPY e2e /e2e

ENV E2E_TASH_BIN=/usr/local/bin/tash
# Enables the container-only real-Zellij screen-capture scenario.
ENV E2E_ALLOW_ZELLIJ=1
WORKDIR /e2e
ENTRYPOINT ["/e2e/run.sh"]
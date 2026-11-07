# syntax=docker/dockerfile:1

# Builds the coordination example binary.
#
# The build context must be the parent directory (pkg/coordination), because
# the example module resolves the unpublished coordination module with
# `replace ... => ../` (see example/go.mod):
#
#   docker build -t coordination-example -f example/Dockerfile ..
#
# Multi-stage and CGO-free: pgx/v5 and nats-server are pure Go.

FROM golang:1.26-alpine AS build

# The context is pkg/coordination: the coordination module lives at the
# context root (go.mod + *.go + postgres/) and the example module in example/.
WORKDIR /src
COPY . .
WORKDIR /src/example
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /bin/coordinator .

FROM alpine:3.22

# /data holds JetStream storage for the slot this process owns (NATS_DATA_DIR).
RUN mkdir -p /data && chown 65534:65534 /data
COPY --from=build --chown=65534:65534 /bin/coordinator /app/coordinator
COPY --chown=65534:65534 example/entrypoint.sh /app/entrypoint.sh

USER 65534:65534
ENTRYPOINT ["/bin/sh", "/app/entrypoint.sh"]

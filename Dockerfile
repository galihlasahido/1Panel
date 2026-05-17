# syntax=docker/dockerfile:1.7
#
# 1Panel built from THIS source, runnable in Docker.
#
# Honest scope: 1Panel is a *host* management panel. In a container the
# Docker / app-store / website / database features work against the
# host's Docker daemon (mount /var/run/docker.sock). Host-OS features
# (system users, firewall/nft, systemd services, SSH, OS updates, disk
# quota) shell out to local binaries and therefore act on the
# CONTAINER, not the host — they are degraded/non-functional here. For
# full host management, run on a real Linux host (privileged + host
# namespaces) or use the upstream host installer.

# ── Stage 1: frontend (embedded into core) ──
FROM node:20-bookworm AS web
WORKDIR /src
COPY frontend/ ./frontend/
# vite build.outDir is ../core/cmd/server/web
RUN mkdir -p core/cmd/server/web
WORKDIR /src/frontend
RUN npm install && npm run build:pro

# ── Stage 2: compile core + agent ──
FROM golang:1.25-bookworm AS build
WORKDIR /src
# core/agent go.mod both `replace .../libpanel => ../libpanel`, so the
# sibling layout must be preserved.
COPY core/ ./core/
COPY agent/ ./agent/
COPY libpanel/ ./libpanel/
# Overlay the built SPA so core's //go:embed picks it up.
COPY --from=web /src/core/cmd/server/web/ ./core/cmd/server/web/
# `//go:embed static/*` needs the dir to be non-empty even if vite
# emitted nothing there.
RUN mkdir -p core/cmd/server/web/static core/cmd/server/web/assets \
    && touch core/cmd/server/web/static/.keep
ENV CGO_ENABLED=0 GOOS=linux
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    cd core && go build -trimpath -ldflags '-s -w' -o /out/1panel-core ./cmd/server/main.go
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    cd agent && go build -trimpath -ldflags '-s -w' -o /out/1panel-agent ./cmd/server/main.go

# ── Stage 3: runtime ──
# Ubuntu (not distroless): the agent shells out to host tooling and
# expects a real userland + the docker CLI.
FROM ubuntu:24.04
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates curl tzdata docker.io \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/1panel-core  /usr/local/bin/1panel-core
COPY --from=build /out/1panel-agent /usr/local/bin/1panel-agent
COPY deploy/docker/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh \
      /usr/local/bin/1panel-core /usr/local/bin/1panel-agent
EXPOSE 9999
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]

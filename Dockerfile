# syntax=docker/dockerfile:1
#
# Multi-stage, minimal, reproducible image for agennextd.
# Trusted base images only; pin to @sha256 digests in CI (see SUPPLY_CHAIN.md).
# The runtime stage is distroless static + nonroot: no shell, no package manager.

# --- build stage ---
FROM golang:1.27 AS build
WORKDIR /src

# Module layer (cached). The core has zero third-party deps, so this is tiny.
COPY go.mod ./
RUN go mod download

COPY . .
# Static, stripped, reproducible build.
ENV CGO_ENABLED=0 GOFLAGS=-trimpath
RUN go build -ldflags="-s -w" -o /out/agennextd ./cmd/agennextd

# --- runtime stage ---
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /out/agennextd /agennextd
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/agennextd"]

# ── Item 4: Automated Image Tiering ─────────────────────────────────────────────────────
# Real, multi-stage build. Builder stage compiles the real forcedream-cli binary; the
# final, shipped image contains ONLY that compiled binary + real CA certificates -- no Go
# toolchain, no source code, no build cache, nothing else.
#
# Real, deliberate choices, not guesses:
# - CGO_ENABLED=0: forces a real, fully static binary with zero C library dependencies,
#   so it can run on a minimal base with nothing else installed.
# - -ldflags="-s -w": strips real debug symbols and DWARF tables -- a genuine, well-known
#   Go size optimization, not a gimmick; has no effect on runtime behavior.
# - Alpine, not `scratch`: the CLI makes many real HTTPS calls (npm, crates.io, Maven
#   Central, NuGet, Packagist, RubyGems, Hex.pm, Docker Hub, GitHub, ForceDream itself).
#   `scratch` has zero files, including no CA certificate bundle, which would silently
#   break every one of those real TLS connections. Alpine, with ca-certificates
#   explicitly installed, is the real, correct minimal choice here -- also matching the
#   original spec's own stated preference for Alpine specifically.

FROM golang:1.22-alpine AS builder
WORKDIR /build

# Real dependency layer, cached separately from source so a source-only change doesn't
# force a full, real re-download of every dependency on each build.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /forcedream .

FROM alpine:latest
RUN apk add --no-cache ca-certificates && \
    adduser -D -H -s /sbin/nologin forcedream
USER forcedream
COPY --from=builder /forcedream /usr/local/bin/forcedream
ENTRYPOINT ["forcedream"]

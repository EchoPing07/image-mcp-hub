# syntax=docker/dockerfile:1

# Builder always runs on the runner's native arch (BUILDPLATFORM) and
# cross-compiles to TARGETARCH with pure Go (CGO disabled) — fast, no QEMU
# needed for the build stage. Only the tiny final stage runs under QEMU.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH="${TARGETARCH}" \
    go build -trimpath -ldflags "-s -w" -o /out/image-mcp-hub .

FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata \
 && addgroup -g 1000 -S app \
 && adduser -u 1000 -S -G app app
COPY --from=builder /out/image-mcp-hub /usr/local/bin/image-mcp-hub
WORKDIR /app
# config.json, images/, stats.json all live under /app/data — one volume to persist.
ENV IMAGE_MCP_HUB_CONFIG=/app/data/config.json
RUN mkdir -p /app/data && chown -R app:app /app
USER app
VOLUME ["/app/data"]
EXPOSE 12300
ENTRYPOINT ["image-mcp-hub"]

# === Stage 0: Build Angular UI ===
# Build the Angular frontend in a lightweight Node.js environment.
FROM node:24-alpine AS angular-builder
WORKDIR /app
COPY src/frontend/ .
RUN npm ci && npm run build --omit=dev


# === Stage 1: Build the Go binary ===
FROM golang:1.26-alpine AS go-builder
WORKDIR /app
COPY src/go.mod src/go.sum ./
RUN go mod download
COPY src/ .
# Frontend must be in place before `go build` so //go:embed picks it up.
COPY --from=angular-builder /app/dist/frontend/browser internal/webui/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /wireguard-pro ./cmd/wireguard-pro


# === Stage 2: Final Runtime Image ===
# No shell-outs left in the app (wgctrl/netlink/nftables are all in-process),
# so the runtime image needs nothing but the binary itself.
FROM gcr.io/distroless/static-debian12 AS runtime

COPY --from=go-builder /wireguard-pro /usr/local/bin/wireguard-pro

ENTRYPOINT ["/usr/local/bin/wireguard-pro"]

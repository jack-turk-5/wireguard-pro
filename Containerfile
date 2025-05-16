# === Stage 0: Build Angular UI ===
FROM node:24-alpine AS angular-builder
WORKDIR /app
COPY src/frontend/ .
RUN npm ci && npm run build --production

# === Stage 1: Build Python venv & BoringTun on Debian-slim ===
FROM python:3.13-slim AS builder
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      gcc build-essential pkg-config libssl-dev cargo \
    && rm -rf /var/lib/apt/lists/* && \
    cargo install boringtun-cli --locked --root /usr/local

WORKDIR /app
COPY requirements.txt .
RUN python3 -m venv /venv && \
    /venv/bin/pip install --no-cache-dir -r requirements.txt

# === Stage 2: Runtime w/ Caddy & Flask ===
FROM python:3.13-slim AS runtime

# Install Caddy and tooling
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      curl gnupg ca-certificates wireguard-tools iproute2 nftables ethtool \
    && rm -rf /var/lib/apt/lists/*

# Add Caddy’s apt repo & install
RUN curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' \
      | gpg --dearmor \
        -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg && \
    printf 'deb [signed-by=/usr/share/keyrings/caddy-stable-archive-keyring.gpg] \
https://dl.cloudsmith.io/public/caddy/stable/deb/debian any-version main\n' \
      > /etc/apt/sources.list.d/caddy-stable.list && \
    apt-get update && \
    apt-get install -y --no-install-recommends caddy \
    && rm -rf /var/lib/apt/lists/*

# Copy Angular build for Caddy
COPY --from=angular-builder /app/dist/frontend/browser /usr/share/caddy/html

# Copy BoringTun, venv, app & bootstrap
COPY --from=builder /usr/local/bin/boringtun-cli /usr/local/bin/
COPY --from=builder /venv /venv

WORKDIR /app
COPY src/ .
COPY container/bootstrap.py /
COPY container/nftables.conf /etc/nftables.conf

ENV PATH="/venv/bin:$PATH"
RUN chmod +x /bootstrap.py

# Copy your Caddyfile
COPY container/Caddyfile /etc/caddy/Caddyfile

ENTRYPOINT ["/bootstrap.py"]
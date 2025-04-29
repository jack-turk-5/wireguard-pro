# === Stage 1: Build Python venv & BoringTun on Debian-slim ===
FROM python:3.13-slim AS builder

# Install build tools
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      gcc build-essential pkg-config libssl-dev cargo \
    && rm -rf /var/lib/apt/lists/*

# Compile BoringTun CLI
RUN cargo install boringtun-cli --locked --root /usr/local

# Create and populate Python venv
WORKDIR /app
COPY requirements.txt .
RUN python3 -m venv /venv && \
    /venv/bin/pip install --no-cache-dir -r requirements.txt

# === Stage 2: Runtime on Debian-slim with Caddy ===
FROM python:3.13-slim AS runtime

# 1. Add Caddy official repo & install Caddy
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      debian-keyring debian-archive-keyring apt-transport-https curl ca-certificates \
    && rm -rf /var/lib/apt/lists/* && \
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' \
      | tee /usr/share/keyrings/caddy-stable-archive-keyring.gpg && \
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' \
      | tee /etc/apt/sources.list.d/caddy-stable.list && \
    apt-get update && apt-get install -y caddy \
    && rm -rf /var/lib/apt/lists/* :contentReference[oaicite:6]{index=6}

# 2. Install runtime dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      bash wireguard-tools socat iproute2 \
    && rm -rf /var/lib/apt/lists/*

# 3. Copy BoringTun, Python venv, app code & bootstrap
COPY --from=builder /usr/local/bin/boringtun-cli /usr/local/bin/
COPY --from=builder /venv /venv
WORKDIR /app
COPY src/ .
COPY container/bootstrap.py /bootstrap.py

ENV PATH="/venv/bin:$PATH"
RUN chmod +x /bootstrap.py

# 4. Copy Caddy config
COPY Caddyfile /etc/caddy/Caddyfile

# 5. Entrypoint: run bootstrap (WireGuard + Gunicorn), then caddy as PID 1
ENTRYPOINT ["/bootstrap.py"]
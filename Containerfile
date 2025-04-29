# === Stage 1: Build Python venv & BoringTun ===
FROM docker.io/python:3.13-slim AS builder

# Install build dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      gcc build-essential pkg-config libssl-dev cargo \
    && rm -rf /var/lib/apt/lists/*

# Compile boringtun-cli
RUN cargo install boringtun-cli --locked --root /usr/local

# Create venv and install Python deps
WORKDIR /app
COPY requirements.txt .
RUN python3 -m venv /venv && \
    /venv/bin/pip install --no-cache-dir -r requirements.txt

# === Stage 2: Runtime using official Caddy image ===
FROM docker.io/caddy:2-alpine AS runtime

# Install runtime tools (WireGuard, socat, iproute2, bash)
RUN apk add --no-cache \
      wireguard-tools socat iproute2 bash

# Copy BoringTun and Python venv from builder
COPY --from=builder /usr/local/bin/boringtun-cli /usr/local/bin/
COPY --from=builder /venv /venv

# Copy application code & bootstrap
WORKDIR /app
COPY src/ .
COPY container/bootstrap.py /bootstrap.py

# Copy Caddy configuration
COPY container/Caddyfile /etc/caddy/Caddyfile

ENV PATH="/venv/bin:$PATH"
RUN chmod +x /bootstrap.py

# Entrypoint: run bootstrap (sets up WG, socat, boringtun, Gunicorn)
#           then exec Caddy in foreground to handle HTTP on FD 3
ENTRYPOINT ["/bin/sh","-c", "/bootstrap.py"]
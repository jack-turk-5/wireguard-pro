# === Stage 1: Build Python venv & BoringTun on Debian-slim ===
FROM python:3.13-slim AS builder

# 1. Install build tools
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      gcc build-essential pkg-config libssl-dev cargo \
    && rm -rf /var/lib/apt/lists/*

# 2. Compile BoringTun CLI
RUN cargo install boringtun-cli --locked --root /usr/local

# 3. Create and populate Python venv
WORKDIR /app
COPY requirements.txt .
RUN python3 -m venv /venv && \
    /venv/bin/pip install --no-cache-dir -r requirements.txt

# === Stage 2: Runtime on Debian-slim with Caddy ===
FROM python:3.13-slim AS runtime

# 1. Install GPG & HTTPS transport
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      curl gnupg ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# 2. Add Cloudsmith GPG key
RUN curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' \
      | gpg --dearmor \
        -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg

# 3. Add Caddy apt source
RUN printf 'deb [signed-by=/usr/share/keyrings/caddy-stable-archive-keyring.gpg] \
  https://dl.cloudsmith.io/public/caddy/stable/deb/debian any-version main\n' \
  > /etc/apt/sources.list.d/caddy-stable.list

# 4. Install Caddy
RUN apt-get update && \
    apt-get install -y --no-install-recommends caddy \
    && rm -rf /var/lib/apt/lists/* && xcaddy build --with \
    github.com/WeidiDeng/caddy-socket-activation

# 5. Install runtime dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      bash wireguard-tools socat iproute2 \
    && rm -rf /var/lib/apt/lists/*

# 6. Copy BoringTun, Python venv, app code & bootstrap
COPY --from=builder /usr/local/bin/boringtun-cli /usr/local/bin/
COPY --from=builder /venv /venv
WORKDIR /app
COPY src/ .
COPY container/bootstrap.py /bootstrap.py

ENV PATH="/venv/bin:$PATH"
RUN chmod +x /bootstrap.py

# 7. Copy Caddy config
COPY container/Caddyfile /etc/caddy/Caddyfile

# 8. Entrypoint: run bootstrap (WireGuard + Gunicorn), then caddy as PID 1
ENTRYPOINT ["/bootstrap.py"]
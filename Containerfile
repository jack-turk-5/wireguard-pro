# === Stage 0: Build Angular UI ===
# Build the Angular frontend in a lightweight Node.js environment.
FROM node:24-alpine AS angular-builder
WORKDIR /app
COPY src/frontend/ .
# Use --no-optional to skip unnecessary packages like puppeteer
RUN npm ci --omit=optional && npm run build --omit=dev


# === Stage 1: Build Python venv, BoringTun, and Caddy ===
# Use a full Python image here to get build tools for dependencies.
FROM python:3.13-slim AS builder

# Install build dependencies for BoringTun and Caddy
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      gcc build-essential pkg-config libssl-dev cargo git curl gnupg ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Configure cargo for faster, non-interactive builds
RUN mkdir -p /.cargo && \
    printf '[net]\ngit-fetch-with-cli = true\n' > /.cargo/config.toml

# Install BoringTun
RUN cargo install boringtun-cli --locked --root /usr/local

# Install Caddy
RUN curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' \
      | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg && \
    printf 'deb [signed-by=/usr/share/keyrings/caddy-stable-archive-keyring.gpg] https://dl.cloudsmith.io/public/caddy/stable/deb/debian any-version main\n' \
      > /etc/apt/sources.list.d/caddy-stable.list && \
    apt-get update && \
    apt-get install -y --no-install-recommends caddy \
    && rm -rf /var/lib/apt/lists/*


# === Stage 2: Final Runtime Image ===
# Use a minimal Debian base image for the final stage.
FROM debian:trixie-slim AS runtime

# Install only the necessary runtime dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      wireguard-tools iproute2 nftables ethtool python3-venv python3-pip build-essential libmnl-dev \
    && rm -rf /var/lib/apt/lists/*

# Copy all artifacts from the builder stages
COPY --from=angular-builder /app/dist/frontend/browser /usr/share/caddy/html
COPY --from=builder /usr/local/bin/boringtun-cli /usr/local/bin/
COPY --from=builder /usr/bin/caddy /usr/bin/caddy

# Copy application code and configs
WORKDIR /app
COPY src/ .
COPY container/bootstrap.py /
COPY container/Caddyfile /etc/caddy/Caddyfile
COPY requirements.txt .

# Create and populate the Python virtual environment
RUN python3 -m venv /venv && \
    /venv/bin/pip install --no-cache-dir -r requirements.txt

# Set up environment
ENV PATH="/venv/bin:/usr/local/bin:$PATH"
ENV GUNICORN_CMD_ARGS="--workers 2 --worker-class uvicorn.workers.UvicornWorker --bind unix:/run/gunicorn.sock"
RUN chmod +x /bootstrap.py

# Set the entrypoint
ENTRYPOINT ["/bootstrap.py"]
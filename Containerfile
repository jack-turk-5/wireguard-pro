# === Stage 1: Build venv & BoringTun ===
FROM python:3.13-slim AS builder

# 1. Install build deps
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      gcc build-essential pkg-config libssl-dev cargo \
    && rm -rf /var/lib/apt/lists/*

# 2. Install BoringTun into /usr/local/bin
RUN cargo install boringtun-cli --locked --root /usr/local

# 3. Prepare Python dependencies
WORKDIR /app
COPY requirements.txt .
RUN python3 -m venv /venv && \
    /venv/bin/pip install --no-cache-dir \
      --only-binary=:all: -r requirements.txt


# === Stage 2: Runtime ===
FROM python:3.13-slim AS runtime

# 1. Install runtime tools only
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      bash wireguard-tools socat iproute2 \
    && rm -rf /var/lib/apt/lists/*

# 2. Copy BoringTun and venv
COPY --from=builder /usr/local/bin/boringtun-cli /usr/local/bin/
COPY --from=builder /venv /venv

# 3. Copy application code & entrypoint
WORKDIR /app
COPY src/ .  
COPY container/bootstrap.py /bootstrap.py

ENV PATH="/venv/bin:$PATH"
RUN chmod +x /bootstrap.py

ENTRYPOINT ["/bootstrap.py"]
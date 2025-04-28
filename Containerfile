# === Stage 1: Builder ===
FROM docker.io/python:3.13-alpine AS builder

RUN apk add --no-cache gcc musl-dev linux-headers libffi-dev openssl-dev cargo

# Set explicit Cargo home for predictable builds
ENV CARGO_HOME=/cargo
ENV PATH=$CARGO_HOME/bin:$PATH

WORKDIR /build

# Build boringtun-cli
RUN cargo install boringtun-cli

# Setup Python virtualenv
COPY requirements.txt .
RUN python3 -m venv venv && \
    . venv/bin/activate && \
    pip install --no-cache-dir -r requirements.txt

COPY src /build/src
COPY container/bootstrap.py /build/bootstrap.py

# === Stage 2: Final Runtime ===
FROM docker.io/python:3.13-alpine

RUN apk add --no-cache bash wireguard-tools socat iproute2

# Copy boringtun binary from builder stage
COPY --from=builder /cargo/bin/boringtun-cli /usr/local/bin/boringtun-cli

# Copy compiled virtualenv and source code
COPY --from=builder /build/venv /venv
COPY --from=builder /build/src /src
COPY --from=builder /build/bootstrap.py /bootstrap.py

WORKDIR /src

# Set venv as default PATH
ENV PATH="/venv/bin:$PATH"

# Explicitly call gunicorn inside venv
ENTRYPOINT ["./venv/bin/gunicorn", "--bind=fd://3", "--workers=4", "--timeout=30", "--graceful-timeout=20", "app:app"]
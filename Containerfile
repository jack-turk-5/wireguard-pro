# === Stage 1: Builder ===
FROM docker.io/python:3.13-alpine AS builder

RUN apk add --no-cache gcc musl-dev linux-headers libffi-dev openssl-dev cargo

ENV CARGO_HOME=/cargo
ENV PATH=$CARGO_HOME/bin:$PATH

WORKDIR /build

RUN cargo install boringtun-cli

COPY requirements.txt .

# (no venv created yet)
COPY src /build/src
COPY container/bootstrap.py /build/bootstrap.py

# === Stage 2: Final Runtime ===
FROM docker.io/python:3.13-alpine

RUN apk add --no-cache bash wireguard-tools socat iproute2

COPY --from=builder /cargo/bin/boringtun-cli /usr/local/bin/boringtun-cli
COPY --from=builder /build/src /src
COPY --from=builder /build/bootstrap.py /bootstrap.py
COPY --from=builder /build/requirements.txt /requirements.txt

WORKDIR /src

# Recreate venv freshly inside final stage
RUN python3 -m venv venv && \
    . venv/bin/activate && \
    pip install --no-cache-dir -r /requirements.txt

ENV PATH="/src/venv/bin:$PATH"

ENTRYPOINT ["/src/venv/bin/gunicorn", "--bind=0.0.0.0:10086", "--workers=4", "--timeout=30", "--graceful-timeout=20", "app:app"]
# === Stage 1: Build venv & BoringTun ===
FROM python:3.13-alpine AS builder
RUN apk add --no-cache gcc musl-dev linux-headers libffi-dev openssl-dev cargo
ENV CARGO_HOME=/cargo PATH=$CARGO_HOME/bin:$PATH
WORKDIR /build
RUN cargo install boringtun-cli --locked --root /usr/local
COPY requirements.txt .
RUN python3 -m venv /venv && \
    /venv/bin/pip install --no-cache-dir \
      --only-binary=:all: -r requirements.txt

# === Stage 2: Runtime ===
FROM python:3.13-alpine
RUN apk add --no-cache \
      bash wireguard-tools socat iproute2 \
      gcompat libstdc++
COPY --from=builder /usr/local/bin/boringtun-cli /usr/local/bin/
COPY --from=builder /build/requirements.txt /requirements.txt
COPY --from=builder /venv /venv
ENV PATH="/venv/bin:$PATH"
COPY src/ /app/
WORKDIR /app
ENTRYPOINT ["/app/bootstrap.py"]
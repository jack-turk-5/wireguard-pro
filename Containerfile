FROM docker.io/python:3.13-alpine

RUN apk add --no-cache bash wireguard-tools boringtun-cli socat iproute2 gcc musl-dev linux-headers

COPY docker/bootstrap.py /bootstrap.py
COPY src /src
COPY requirements.txt /requirements.txt

WORKDIR /src

RUN python3 -m venv venv && \
    . venv/bin/activate && \
    pip install --no-cache-dir -r /requirements.txt

ENTRYPOINT ["./venv/bin/gunicorn", "--bind", "fd://3", "--workers=4", "--timeout=30", "--graceful-timeout=20", "app:app"]

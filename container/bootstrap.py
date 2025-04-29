#!/usr/bin/env python3
import os
import subprocess
import time
import sys

# Only close stray FDs (>=5) to preserve systemd-passed sockets on fd 3 & 4
try:
    max_fd = os.sysconf('SC_OPEN_MAX')
except (AttributeError, ValueError):
    max_fd = 256
for fd in range(5, max_fd):
    try:
        os.close(fd)
    except OSError:
        pass

# Generate WireGuard config if missing
wg_conf = '/etc/wireguard/wg0.conf'
secret_path = '/run/secrets/wg_privatekey'
if not os.path.isfile(wg_conf):
    os.makedirs('/etc/wireguard', exist_ok=True)
    if os.path.exists(secret_path):
        with open(secret_path) as f:
            private_key = f.read().strip()
    else:
        private_key = subprocess.check_output(['wg', 'genkey']).decode().strip()
    with open('/etc/wireguard/privatekey', 'w') as f:
        f.write(private_key)
    pubkey = subprocess.check_output(['wg', 'pubkey'], input=private_key.encode()).decode().strip()
    with open('/etc/wireguard/publickey', 'w') as f:
        f.write(pubkey)
    with open(wg_conf, 'w') as f:
        f.write(f"""[Interface]
PrivateKey = {private_key}
Address = 10.8.0.1/24, fd86:ea04:1111::1/64
ListenPort = 51820
SaveConfig = true
""")


# Detect socket-activation FDs via env var:
listen_fds = int(os.environ.get('LISTEN_FDS', '0'))
if listen_fds < 2:
    sys.stderr.write(f"Error: expected 2 activated sockets, got {listen_fds}\n")
    sys.exit(1)

# systemd sockets start at FD 3
HTTP_FD = 3
UDP_FD  = 4

# Start BoringTun on the UDP FD
boringtun_cmd = [
    'boringtun-cli',
    '--foreground',
    'wg0',
    '--userspace-socket-fd', str(UDP_FD)
]
boringtun_proc = subprocess.Popen(boringtun_cmd, env=os.environ)

# Give BoringTun a moment
time.sleep(0.2)

# Exec Gunicorn on the HTTP FD
os.execv('/src/venv/bin/gunicorn', [
    '/src/venv/bin/gunicorn',
    '--preload',
    '--bind', f'fd://{HTTP_FD}',
    '--workers', '4',
    '--timeout', '30',
    '--graceful-timeout', '20',
    '--reuse-port',
    'app:app'
])
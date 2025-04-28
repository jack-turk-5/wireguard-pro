#!/usr/bin/env python3
import os
import subprocess
import sys
import time

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

# Start Boringtun in background using FD 4 for UDP
boringtun_cmd = [
    'boringtun-cli',
    '--foreground',
    'wg0',
    '--userspace-socket-fd', '4'
]
boringtun_proc = subprocess.Popen(boringtun_cmd, env=os.environ)

# Give Boringtun a moment to set up
time.sleep(0.2)

# Exec Gunicorn binding to FD 3 for HTTP API
os.execv('/src/venv/bin/gunicorn', [
    '/src/venv/bin/gunicorn',
    '--preload',
    '--bind', 'fd://3',
    '--workers', '4',
    '--timeout', '30',
    '--graceful-timeout', '20',
    '--reuse-port',
    'app:app'
])
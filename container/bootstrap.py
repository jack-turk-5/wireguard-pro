#!/usr/bin/env python3
import os
import sys
import subprocess
import time

# ── 1. Close stray FDs (>=5) to avoid leakage ─────────────
try:
    max_fd = os.sysconf('SC_OPEN_MAX')
except (AttributeError, ValueError):
    max_fd = 256

for fd in range(5, max_fd):
    try:
        os.close(fd)
    except OSError:
        pass

# ── 2. Generate WireGuard config if it doesn’t exist ──────
WG_CONF = '/etc/wireguard/wg0.conf'
SECRET_PATH = '/run/secrets/wg_privatekey'

if not os.path.isfile(WG_CONF):
    os.makedirs(os.path.dirname(WG_CONF), exist_ok=True)

    # Load or generate private key
    if os.path.exists(SECRET_PATH):
        with open(SECRET_PATH, 'r') as f:
            private_key = f.read().strip()
    else:
        private_key = subprocess.check_output(['wg', 'genkey']).decode().strip()

    # Write private & public keys
    with open('/etc/wireguard/privatekey', 'w') as f:
        f.write(private_key)
    pubkey = subprocess.check_output(
        ['wg', 'pubkey'],
        input=private_key.encode()
    ).decode().strip()
    with open('/etc/wireguard/publickey', 'w') as f:
        f.write(pubkey)

    # Write wg0.conf
    with open(WG_CONF, 'w') as f:
        f.write(f"""[Interface]
PrivateKey = {private_key}
Address    = 10.8.0.1/24, fd86:ea04:1111::1/64
ListenPort = 51820
SaveConfig = true
""")

# ── 3. Check for socket‐activation FDs ───────────────────
listen_fds = int(os.environ.get('LISTEN_FDS', '0'))
if listen_fds < 2:
    sys.stderr.write(
        f"Error: expected 2 activated sockets, got {listen_fds}\n"
    )
    sys.exit(1)

# systemd passes:
#   FD 3 → TCP socket for Gunicorn
#   FD 4 → UDP socket for WireGuard

HTTP_FD = 3
UDP_FD  = 4

# ── 4. Proxy FD 4 → UDP port 51820 via socat ─────────────
#    This lets boringtun-cli bind port 51820 itself (from wg0.conf).
socat_cmd = [
    'socat',
    f'FD:{UDP_FD}',
    'UDP4-LISTEN:51820,reuseaddr,fork'
]
subprocess.Popen(socat_cmd, env=os.environ)

# ── 5. Launch boringtun-cli (it reads wg0.conf → port 51820) ─
boringtun_proc = subprocess.Popen(
    ['boringtun-cli', '--foreground', 'wg0'],
    env=os.environ
)

# Short pause to let BoringTun initialize
time.sleep(0.2)

# ── 6. Exec Gunicorn on the HTTP FD ────────────────────────
os.execv(
    '/venv/bin/gunicorn',
    [
        '/venv/bin/gunicorn',
        '--preload',
        '--bind', 
        f'fd://{HTTP_FD}',
        '--workers',   
        '4',
        '--timeout',   
        '30',
        '--graceful-timeout', 
        '20',
        '--reuse-port',
        'app:app'
    ]
)
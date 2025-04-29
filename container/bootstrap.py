#!/usr/bin/env python3
import os
import subprocess
import time
import sys

# ── 1. Close stray FDs (>=5) ──────────────────────────────────
try:
    max_fd = os.sysconf('SC_OPEN_MAX')
except (AttributeError, ValueError):
    max_fd = 256
for fd in range(5, max_fd):
    try:
        os.close(fd)
    except OSError:
        pass

# ── 2. Generate WireGuard config if missing ────────────────
WG_CONF     = '/etc/wireguard/wg0.conf'
SECRET_PATH = '/run/secrets/wg_privatekey'

if not os.path.isfile(WG_CONF):
    os.makedirs(os.path.dirname(WG_CONF), exist_ok=True)

    # Load or generate private key
    if os.path.exists(SECRET_PATH):
        with open(SECRET_PATH, 'r') as f:
            private_key = f.read().strip()
    else:
        private_key = subprocess.check_output(['wg', 'genkey']).decode().strip()

    # Write keys
    with open('/etc/wireguard/privatekey', 'w') as f:
        f.write(private_key)
    pubkey = subprocess.check_output(
        ['wg', 'pubkey'], input=private_key.encode()
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

# ── 3. Proxy HTTP (10086 → 0.0.0.0:8000) via socat ─────────
http_socat = subprocess.Popen([
    'socat',
    'TCP4-LISTEN:10086,reuseaddr,fork',    # listen publicly
    'TCP4:0.0.0.0:10086'                   # forward to Gunicorn
], env=os.environ)

# ── 4. Proxy UDP (51820 → 0.0.0.0:51820) via socat ────────
udp_socat = subprocess.Popen([
    'socat',
    'UDP4-LISTEN:51820,reuseaddr,fork',     # listen publicly
    'UDP4:0.0.0.0:51820'                  # forward to BoringTun
], env=os.environ)

# ── 5. Start BoringTun (binds to 0.0.0.0:51820) ───────────
boringtun_proc = subprocess.Popen(
    ['boringtun-cli', '--foreground', 'wg0'],
    env=os.environ
)

# Short pause for BoringTun to initialize
time.sleep(0.2)

# ── 6. Exec Gunicorn on 0.0.0.0:10068 ─────────────────────
os.execv(
    '/src/venv/bin/gunicorn',
    [
        '/src/venv/bin/gunicorn',
        '--preload',
        '--bind',            
        '0.0.0.0:10068',
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
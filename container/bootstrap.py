#!/usr/bin/env python3
import os
import subprocess
import sys
from datetime import time

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
    # Write keys
    with open('/etc/wireguard/privatekey', 'w') as f:
        f.write(private_key)
    pubkey = subprocess.check_output(['wg', 'pubkey'], input=private_key.encode()).decode().strip()
    with open('/etc/wireguard/publickey', 'w') as f:
        f.write(pubkey)
    # Create minimal config
    with open(wg_conf, 'w') as f:
        f.write(f"""[Interface]
PrivateKey = {private_key}
Address = 10.8.0.1/24, fd86:ea04:1111::1/64
ListenPort = 51820
SaveConfig = true
""")

# Exec gunicorn
os.execv('/src/venv/bin/gunicorn', [
    'gunicorn',
    '--preload',
    '--bind', '0.0.0.0:10086',
    '--workers', '4',
    '--timeout', '30',
    '--graceful-timeout', '20',
    '--reuse-port',
    'app:app'
])
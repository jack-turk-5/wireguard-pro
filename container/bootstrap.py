#!/usr/bin/env python3
import os
from os import path, makedirs, environ, execv
from subprocess import check_output, Popen, PIPE, run
from time import sleep
from shutil import which

# Paths
WG_CONF = '/etc/wireguard/wg0.conf'
PRIVATE_KEY_FILE = '/etc/wireguard/privatekey'
SECRET = '/run/secrets/wg-privatekey'

# 1) Initial config generation
if not path.isfile(WG_CONF):
    makedirs(path.dirname(WG_CONF), exist_ok=True)
    if path.exists(SECRET):
        priv = open(SECRET).read().strip()
    else:
        priv = check_output(['wg', 'genkey']).decode().strip()
    # Persist private key
    with open(PRIVATE_KEY_FILE, 'w') as f:
        f.write(priv + '\n')
    # Derive public key
    proc = Popen(
        ['wg', 'pubkey'], stdin=PIPE, stdout=PIPE, stderr=PIPE
    )
    pub_bytes, err_bytes = proc.communicate(priv.encode())
    if proc.returncode != 0:
        raise RuntimeError(f"wg pubkey failed: {err_bytes.decode()}")
    pub = pub_bytes.decode().strip()
    # Write config file
    config_lines = [
        '[Interface]',
        f'PrivateKey = {priv}',
        'Address = 10.8.0.1/24',
        'Address = fd86:ea04:1111::1/64',
        'ListenPort = 51820',
    ]
    with open(WG_CONF, 'w', newline='\n') as f:
        f.write("\n".join(config_lines) + "\n")

# 2) UDP relay for BoringTun
Popen([
    'socat',
    'UDP4-LISTEN:51820,bind=0.0.0.0,reuseaddr,fork',
    'UDP4:0.0.0.0:51820'
], close_fds=False)

# 3) Post-up NAT rules (env)
post_up = os.environ.get('WG_POST_UP')
if post_up:
    Popen(post_up, shell=True)

# 4) Start BoringTun CLI (sets up wg0 tunnel)
environ.setdefault('WG_SUDO', '1')
boringtun = Popen([
    '/usr/local/bin/boringtun-cli', '--foreground', 'wg0'
], close_fds=False)

# 5) Configure interface in kernel
# 5a) load private key into interface
run(['wg', 'set', 'wg0', 'private-key', open(SECRET).read().strip()], check=True)
# 5b) apply entire WG_CONF (addresses, port)
run(['wg', 'setconf', 'wg0', WG_CONF], check=True)

# brief pause for tun to settle
sleep(0.2)

# 6) Launch Gunicorn
Popen([
    '/venv/bin/gunicorn',
    '--preload',
    '--bind', '0.0.0.0:51819',
    '--workers', '4',
    '--timeout', '30',
    '--graceful-timeout', '20',
    '--reuse-port',
    'app:app'
])

# 7) Hand off to Caddy as PID 1 for HTTP UI
caddy_path = which('caddy')
execv(caddy_path, [
    'caddy', 'run',
    '--config', '/etc/caddy/Caddyfile',
    '--adapter', 'caddyfile'
])
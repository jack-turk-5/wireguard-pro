#!/usr/bin/env python3
from secrets import token_urlsafe
from os import environ, path, makedirs, execv
from subprocess import CalledProcessError, check_output, Popen, PIPE, run
from shutil import which

# Flask SECRET_KEY bootstrapping
FLASK_SECRET_FILE = '/data/flask_secret'
makedirs(path.dirname(FLASK_SECRET_FILE), exist_ok=True)

if 'SECRET_KEY' not in environ:
    if path.exists(FLASK_SECRET_FILE):
        environ['SECRET_KEY'] = open(FLASK_SECRET_FILE, 'r').read().strip()
    else:
        key = token_urlsafe(32)
        with open(FLASK_SECRET_FILE, 'w') as f:
            f.write(key)
        environ['SECRET_KEY'] = key

# WireGuard keygen & wg0.conf bootstrapping
WG_CONF, SECRET = '/etc/wireguard/wg0.conf', '/run/secrets/wg-privatekey'
if not path.isfile(WG_CONF):
    makedirs(path.dirname(WG_CONF), exist_ok=True)
    if path.exists(SECRET):
        priv = open(SECRET).read().strip()
    else:
        priv = check_output(['wg','genkey']).decode().strip()
    open('/etc/wireguard/privatekey','w').write(priv)
    proc = Popen(['wg','pubkey'], stdin=PIPE, stdout=PIPE, stderr=PIPE)
    pub_bytes, err_bytes = proc.communicate(priv.encode())
    if proc.returncode != 0:
        raise RuntimeError(f"wg pubkey failed: {err_bytes.decode()}")
    config = [
        "[Interface]",
        f"PrivateKey = {priv}",
        "Address = 10.8.0.1/24",
        "Address = fd86:ea04:1111::1/64",
        "ListenPort = 51820",
        "MTU = 1420"
    ]
    with open(WG_CONF, 'w', newline='\n') as f:
        f.write("\n".join(config) + "\n")

if environ.get('WG_SOCKET_FD', None) is None:
    environ['WG_SOCKET_FD'] = '4'

# Tear down any old wg0, bring up fresh
run(['wg-quick','down','wg0'], check=False)
run(['ip','link','delete','wg0'], check=False)
run(['ip','addr','flush','dev','wg0'], check=False)
try:
    run(['wg-quick','up','wg0'], check=True)
except CalledProcessError as e:
    raise RuntimeError(f"WireGuard failed: {e.stderr}") from e

# Launch Gunicorn on 51818
Popen([
    '/venv/bin/gunicorn',
    '--preload',
    '--bind', '0.0.0.0:51818',
    '--workers', '4',
    '--timeout', '30',
    '--graceful-timeout', '20',
    '--reuse-port',
    'app:app'
], env=environ.copy())

# Apply nftables
run(['sysctl', '-w', 'net.ipv4.ip_forward=1'], check=True)
run(['sysctl', '-w', 'net.ipv6.conf.all.forwarding=1'], check=True)
run(['nft','-f','/etc/nftables.conf'], check=True)

# Exec into Caddy as PID 1
caddy = which('caddy')
execv(caddy, [
    'caddy','run',
    '--config','/etc/caddy/Caddyfile',
    '--adapter','caddyfile'
])
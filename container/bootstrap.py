#!/usr/bin/env python3
from os import path, makedirs, execv
from subprocess import check_output, Popen, PIPE, run
from shutil import which

# ——— WireGuard keygen & wg0.conf bootstrapping ———
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
    ]
    with open(WG_CONF, 'w', newline='\n') as f:
        f.write("\n".join(config) + "\n")

# Tear down any old wg0, bring up fresh
run(['wg-quick','down','wg0'], check=False)
run(['ip','link','delete','wg0'], check=False)
run(['ip','addr','flush','dev','wg0'], check=False)
run(['wg-quick','up','wg0'], check=True)

# Launch Gunicorn on 51818
Popen([
    '/venv/bin/gunicorn',
    '--preload',
    '--bind','0.0.0.0:51818',
    '--workers','4',
    '--timeout','30',
    '--graceful-timeout','20',
    '--reuse-port',
    'app:app'
])

# Apply nftables + ethtool tweaks
run(['nft','-f','/etc/nftables.conf'], check=True)
run(['ethtool','-K','tap0','gro','on','gso','on','ufo','on'], check=True)

# Exec into Caddy as PID 1
caddy = which('caddy')
execv(caddy, [
    'caddy','run',
    '--config','/etc/caddy/Caddyfile',
    '--adapter','caddyfile'
])
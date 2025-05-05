#!/usr/bin/env python3
from os import path, makedirs, execv
from subprocess import check_output, Popen, PIPE, run
from shutil import which


# Load or create public and private keys for server
WG_CONF, SECRET = '/etc/wireguard/wg0.conf', '/run/secrets/wg-privatekey'
if not path.isfile(WG_CONF):
    makedirs(path.dirname(WG_CONF), exist_ok=True)
    if path.exists(SECRET):
        priv = open(SECRET).read().strip()
    else:
        priv = check_output(['wg','genkey']).decode().strip()
    open('/etc/wireguard/privatekey','w').write(priv)
    proc = Popen(
        ['wg', 'pubkey'],
        stdin=PIPE,
        stdout=PIPE,
        stderr=PIPE,
    )
    pub_bytes, err_bytes = proc.communicate(priv.encode())
    if proc.returncode != 0:
        raise RuntimeError(f"wg pubkey failed: {err_bytes.decode()}")
    pub = pub_bytes.decode().strip()
    # This syntax is mega sensitive, don't touch unless necessary
    config_lines = [
        "[Interface]",
        f"PrivateKey = {priv}",
        "Address = 10.8.0.1/24",
        "Address = fd86:ea04:1111::1/64",
        "ListenPort = 51820",
    ]
    with open(WG_CONF, 'w', newline='\n') as f:
        f.write("\n".join(config_lines) + "\n")

# Best‑effort down (may skip deletion if not a kernel WG iface)
run(['wg-quick', 'down', 'wg0'], check=False)
# Force‑delete any old wg0 link (works for both TUN and wireguard types)
run(['ip', 'link', 'delete', 'wg0'], check=False)
# Flush leftover IP addresses
run(['ip', 'addr', 'flush', 'dev', 'wg0'], check=False)
# Now bring up clean wg0
run(['wg-quick', 'up', 'wg0'], check=True)

# Launch Gunicorn in background
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

# Bring up nftables firewall rules
run(
    [
        "nft",
        "-f",
        "/etc/nftables.conf"
    ],
    check=True
)

# Optimize tap0 (slirp4netns generated interface)
run([
        "ethtool",
        "-K",
        "tap0",
        "gro", "on",
        "gso", "on",
        "ufo", "on"
     ],
    check=True)

# Hand off to Caddy as PID 1
caddy_path = which('caddy')
execv(caddy_path,
         [
             'caddy',
             'run',
             '--config',
             '/etc/caddy/Caddyfile',
             '--adapter',
             'caddyfile'
         ]
)
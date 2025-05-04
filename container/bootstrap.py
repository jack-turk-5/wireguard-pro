#!/usr/bin/env python3
from os import path, makedirs, execv
from subprocess import check_output, Popen, PIPE, run
from shutil import which

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
    # write a WG_CONF that BoringTun will actually parse
    # assemble exactly the lines you want
    config_lines = [
        "[Interface]",
        f"PrivateKey = {priv}",
        "Address = 10.8.0.1/24",
        "Address = fd86:ea04:1111::1/64",
        "ListenPort = 51820",
    ]
    with open(WG_CONF, 'w', newline='\n') as f:
        f.write("\n".join(config_lines) + "\n")

# 1) Best‑effort down (may skip deletion if not a kernel WG iface)
run(['wg-quick', 'down', 'wg0'], check=False)
# 2) Force‑delete any wg0 link (works for both TUN and wireguard types)
run(['ip', 'link', 'delete', 'wg0'], check=False)
# 3) Flush leftover IP addresses
run(['ip', 'addr', 'flush', 'dev', 'wg0'], check=False)
# 4) Now bring up clean
run(['wg-quick', 'up', 'wg0'], check=True)

Popen([
    'socat',
    '-u', '-d', '-d',
    'FD:4',
    'UDP:127.0.0.1:51820'
], close_fds=False)

Popen([
    'socat',
    '-u', '-d', '-d',
    'UDP-CONNECT:127.0.0.1:51820',
    'FD:4'
], close_fds=False)

# 4) Launch Gunicorn in background
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

run(
    [
        "nft",
        "-f",
        "/etc/nftables.conf"
    ],
    check=True
)



# Step 5: Hand off to Caddy as PID 1
caddy_path = which('caddy')  # should resolve to /usr/bin/caddy
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
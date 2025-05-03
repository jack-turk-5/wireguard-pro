#!/usr/bin/env python3
from os import path, makedirs, environ, execv
from subprocess import check_output, Popen, PIPE
from time import sleep
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
        f"PrivateKey {priv}",             # <- exactly one space here
        "Address 10.8.0.1/24",
        "Address fd86:ea04:1111::1/64",
        "ListenPort 51820",
        ""                                 # final blank line
    ]
    conf = "\n".join(config_lines)
    with open(WG_CONF, 'w') as f:
            f.write(conf)
    print("Wrote wg0.conf:\n" + conf)

# 2) UDP relay for BoringTun
Popen(
    ['socat',
     'UDP4-LISTEN:51820,bind=0.0.0.0,reuseaddr,fork',
     'UDP4:0.0.0.0:51820'],
    close_fds=False
)
# 3) Start BoringTun CLI in foreground (inherits FD 4)
environ.setdefault('WG_SUDO', '1')
Popen(
    ['/usr/local/bin/boringtun-cli', '--foreground', 'wg0'],
    close_fds=False
)
sleep(0.2)


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
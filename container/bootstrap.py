#!/usr/bin/env python3
import os, subprocess, time


WG_CONF, SECRET = '/etc/wireguard/wg0.conf', '/run/secrets/wg-privatekey'
if not os.path.isfile(WG_CONF):
    os.makedirs(os.path.dirname(WG_CONF), exist_ok=True)
    if os.path.exists(SECRET):
        priv = open(SECRET).read().strip()
    else:
        priv = subprocess.check_output(['wg','genkey']).decode().strip()
    open('/etc/wireguard/privatekey','w').write(priv)
    proc = subprocess.Popen(
        ['wg', 'pubkey'],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    pub_bytes, err_bytes = proc.communicate(priv.encode())
    if proc.returncode != 0:
        raise RuntimeError(f"wg pubkey failed: {err_bytes.decode()}")
    pub = pub_bytes.decode().strip()
    open('/etc/wireguard/publickey','w').write(pub)
    open(WG_CONF,'w').write(f"""[Interface]
PrivateKey = {priv}
Address    = 10.8.0.1/24, fd86:ea04:1111::1/64
ListenPort = 51820
SaveConfig = true
""")

# 2) UDP relay for BoringTun
subprocess.Popen(
    ['socat',
     'UDP4-LISTEN:51820,bind=0.0.0.0,reuseaddr,fork',
     'UDP4:0.0.0.0:51820'],
    close_fds=False
)
# 3) Start BoringTun CLI in foreground (inherits FD 4)
os.environ.setdefault('WG_SUDO', '1')
subprocess.Popen(
    ['/usr/local/bin/boringtun-cli', '--foreground', 'wg0'],
    close_fds=False
)
time.sleep(0.2)


# 4) Launch Gunicorn in background, binding to localhost:51819
subprocess.Popen([
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
os.execv(
    '/usr/local/bin/caddy',
    ['caddy', 'run', '--config', '/etc/caddy/Caddyfile', '--adapter', 'caddyfile']
)
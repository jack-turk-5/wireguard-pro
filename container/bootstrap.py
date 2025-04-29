#!/usr/bin/env python3
import os, subprocess, time, sys

# 1. Close stray FDs (>=5)
try:
    max_fd = os.sysconf('SC_OPEN_MAX')
except:
    max_fd = 256
for fd in range(5, max_fd):
    try: os.close(fd)
    except OSError: pass

# 2. Generate WireGuard config if missing
WG_CONF, SECRET = '/etc/wireguard/wg0.conf', '/run/secrets/wg_privatekey'
if not os.path.isfile(WG_CONF):
    os.makedirs(os.path.dirname(WG_CONF), exist_ok=True)
    if os.path.exists(SECRET):
        priv = open(SECRET).read().strip()
    else:
        priv = subprocess.check_output(['wg','genkey']).decode().strip()
    open('/etc/wireguard/privatekey','w').write(priv)
    pub = subprocess.check_output(
        ['wg','pubkey'], input=priv.encode()
    ).decode().strip()
    open('/etc/wireguard/publickey','w').write(pub)
    open(WG_CONF,'w').write(f"""[Interface]
PrivateKey = {priv}
Address    = 10.8.0.1/24, fd86:ea04:1111::1/64
ListenPort = 51820
SaveConfig = true
""")

# 3. Proxy HTTP: public 10086 → local 8000
subprocess.Popen([
    'socat',
    'TCP4-LISTEN:10086,bind=0.0.0.0,reuseaddr,fork',
    'TCP4:127.0.0.1:10086'
], env=os.environ)

# 4. Proxy UDP: public 51820 → local 51820
subprocess.Popen([
    'socat',
    'UDP4-LISTEN:51820,bind=0.0.0.0,reuseaddr,fork',
    'UDP4:127.0.0.1:51820'
], env=os.environ)

# 5. Start BoringTun (internal UDP bind)
subprocess.Popen(
    ['boringtun-cli', '--foreground', 'wg0'],
    env=os.environ
)

time.sleep(0.2)  # let BoringTun settle

# 6. Exec Gunicorn on internal HTTP bind
os.execv(
    '/src/venv/bin/gunicorn',
    [
        '/src/venv/bin/gunicorn',
        '--preload',
        '--bind', '127.0.0.1:10086',
        '--workers', '4',
        '--timeout', '30',
        '--graceful-timeout', '20',
        '--reuse-port',
        'app:app'
    ]
)
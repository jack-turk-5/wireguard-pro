#!/usr/bin/env python3
import os, subprocess, time

# 1. Close stray FDs (>=5)
try:
    max_fd = os.sysconf('SC_OPEN_MAX')
except:
    max_fd = 256
for fd in range(5, max_fd):
    try: os.close(fd)
    except OSError: pass

# 2. Generate wg0.conf & keys if missing (unchanged)
WG_CONF, SECRET = '/etc/wireguard/wg0.conf', '/run/secrets/wg_privatekey'
if not os.path.isfile(WG_CONF):
    os.makedirs(os.path.dirname(WG_CONF), exist_ok=True)
    if os.path.exists(SECRET):
        private = open(SECRET).read().strip()
    else:
        private = subprocess.check_output(['wg','genkey']).decode().strip()
    open('/etc/wireguard/privatekey','w').write(private)
    pub = subprocess.check_output(['wg','pubkey'], input=private.encode()).decode().strip()
    open('/etc/wireguard/publickey','w').write(pub)
    open(WG_CONF,'w').write(f"""[Interface]
PrivateKey = {private}
Address    = 10.8.0.1/24, fd86:ea04:1111::1/64
ListenPort = 51820
SaveConfig = true
""")

# 3. Proxy HTTP: public 10086 → local 10068
http_socat = subprocess.Popen([
    'socat',
    'TCP4-LISTEN:10086,bind=0.0.0.0,reuseaddr,fork',  # listen on all IPv4:10086
    'TCP4:127.0.0.1:10068'                             # forward to Gunicorn’s port
], env=os.environ)

# 4. Proxy UDP: public 51820 → local 51820
udp_socat = subprocess.Popen([
    'socat',
    'UDP4-LISTEN:51820,bind=0.0.0.0,reuseaddr,fork',  # listen on all IPv4 UDP:51820
    'UDP4:127.0.0.1:51820'                            # forward to BoringTun’s port
], env=os.environ)

# 5. Start BoringTun (binds on 127.0.0.1:51820 per wg0.conf)
boringtun = subprocess.Popen(
    ['boringtun-cli','--foreground','wg0'],
    env=os.environ
)

time.sleep(0.2)  # allow BoringTun to initialize

# 6. Exec Gunicorn on local port 10068
os.execv(
    '/venv/bin/gunicorn',
    [
        '/venv/bin/gunicorn',
        '--preload',
        '--bind','127.0.0.1:10068',  # local bind for socat proxy
        '--workers','4',
        '--timeout','30',
        '--graceful-timeout','20',
        '--reuse-port',
        'app:app'
    ]
)
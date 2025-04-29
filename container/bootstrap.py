#!/usr/bin/env python3
import os, subprocess, time


WG_CONF, SECRET = '/etc/wireguard/wg0.conf', '/run/secrets/wg_privatekey'
if not os.path.isfile(WG_CONF):
    os.makedirs(os.path.dirname(WG_CONF), exist_ok=True)
    if os.path.exists(SECRET):
        priv = open(SECRET).read().strip()
    else:
        priv = subprocess.check_output(['wg','genkey']).decode().strip()
    open('/etc/wireguard/privatekey','w').write(priv)
    pub = subprocess.check_output(f'wg pubkey {priv.encode()}').decode().strip()
    open('/etc/wireguard/publickey','w').write(pub)
    open(WG_CONF,'w').write(f"""[Interface]
PrivateKey = {priv}
Address    = 10.8.0.1/24, fd86:ea04:1111::1/64
ListenPort = 51820
SaveConfig = true
""")
'''
subprocess.Popen([
    'socat',
    'TCP4-LISTEN:51819,bind=0.0.0.0,reuseaddr,fork',
    'TCP4:127.0.0.1:51819'
], env=os.environ)
'''
subprocess.Popen([
    'socat',
    'UDP4-LISTEN:51820,bind=0.0.0.0,reuseaddr,fork',
    'UDP4:127.0.0.1:51820'
], env=os.environ)

subprocess.Popen(
    ['boringtun-cli', '--foreground', 'wg0', '&'],
    env=os.environ
)

time.sleep(0.2)

os.execv(
    '/venv/bin/gunicorn',
    [
        '--preload',
        '--daemon', False,
        '--bind', 'fd://3',
        '--workers', '4',
        '--timeout', '30',
        '--graceful-timeout', '20',
        '--reuse-port',
        'app:app'
    ]
)
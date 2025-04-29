#!/usr/bin/env python3
import os, subprocess, time, fcntl


os.environ.setdefault('LISTEN_FDS', os.environ.get('LISTEN_FDS', '2'))
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

# UDP relay for BoringTun
subprocess.Popen(
    [
        'socat',
        'UDP4-LISTEN:51820,bind=127.0.0.1,reuseaddr,fork',
        'UDP4:127.0.0.1:51820'
    ],
    env=os.environ,
    close_fds=False
)

# Start BoringTun in-foreground on wg0, inherit fds
os.environ.setdefault('WG_SUDO', '1')
subprocess.Popen(
    [
        '/usr/local/bin/boringtun-cli', '--foreground', 'wg0'
    ],
    env=os.environ,
    close_fds=False
)  # keep FD 4 open

time.sleep(0.2)
# ─── hide the UDP FD from Gunicorn ──────────────────────────────────────────
# Parse how many sockets we received
listen_fds = int(os.environ.get('LISTEN_FDS', '0'))
# If there were 2 (TCP + UDP), close the UDP one on FD 4
if listen_fds > 1:
    try:
        os.close(4)
    except OSError:
        pass

# ─── un-set close-on-exec on the remaining socket FDs ─────────────────────────
import fcntl
for fd in range(3, 3 + listen_fds):
    try:
        flags = fcntl.fcntl(fd, fcntl.F_GETFD)
        # clear the FD_CLOEXEC bit so execv() won’t close it
        fcntl.fcntl(fd, fcntl.F_SETFD, flags & ~fcntl.FD_CLOEXEC)
    except OSError:
        pass

os.execv(
    '/venv/bin/gunicorn',
    [
            '/venv/bin/gunicorn',
            '--preload',
            '--bind', 'fd://3',
            '--workers', '4',
            '--timeout', '30',
            '--graceful-timeout', '20',
            '--reuse-port',
            'app:app'
    ]
)
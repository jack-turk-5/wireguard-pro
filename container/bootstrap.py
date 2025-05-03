#!/usr/bin/env python3
from signal import signal, SIGINT, SIGTERM
from os import path, makedirs, environ
from sys import exit
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
        f"PrivateKey = {priv}",
        "Address = 10.8.0.1/24",
        "Address = fd86:ea04:1111::1/64",
        "ListenPort = 51820",
    ]
    with open(WG_CONF, 'w', newline='\n') as f:
        f.write("\n".join(config_lines) + "\n")

# 2) UDP relay for BoringTun
Popen(
    ['socat',
     'UDP4-LISTEN:51820,bind=0.0.0.0,reuseaddr,fork',
     'UDP4:0.0.0.0:51820'],
    close_fds=False
)

post_up = environ.get("WG_POST_UP")
if post_up:
    Popen(post_up, shell=True)

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

caddy_path = which('caddy')
# 1) Spawn Caddy (instead of execv)
caddy = Popen(
    [caddy_path, 'run', '--config', '/etc/caddy/Caddyfile', '--adapter', 'caddyfile']
)

# 2) Define cleanup
def cleanup_and_exit(signum=None, frame=None):
    post_down = environ.get("WG_POST_DOWN")
    if post_down:
        # run the post‑down rules in a shell
        Popen(post_down, shell=True).wait()
    # terminate Caddy if it’s still running
    if caddy.poll() is None:
        caddy.terminate()
        caddy.wait()
    exit(0)

# 3) Hook signals
signal(SIGTERM, cleanup_and_exit)
signal(SIGINT,  cleanup_and_exit)

# 4) Optionally also on normal exit
import atexit
atexit.register(cleanup_and_exit)

# 5) Wait for Caddy to exit (so Python stays alive to catch signals)
caddy.wait()
# if Caddy exits on its own, run cleanup too
cleanup_and_exit()
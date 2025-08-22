#!/usr/bin/env python3
import os
import subprocess
import sys
from secrets import token_urlsafe
import time

def run_command(command, check=True):
    """Helper to run a command and log its output."""
    print(f"Running command: {' '.join(command)}")
    try:
        subprocess.run(command, check=check)
    except subprocess.CalledProcessError as e:
        print(f"Command failed: {e}")
        sys.exit(1)

def setup_secret_key():
    """Set up the application's secret key."""
    secret_file = '/data/app_secret'
    if 'SECRET_KEY' not in os.environ:
        if os.path.exists(secret_file):
            with open(secret_file, 'r') as f:
                os.environ['SECRET_KEY'] = f.read().strip()
        else:
            key = token_urlsafe(32)
            os.makedirs(os.path.dirname(secret_file), exist_ok=True)
            with open(secret_file, 'w') as f:
                f.write(key)
            os.environ['SECRET_KEY'] = key

def get_named_socket_fds():
    """Get file descriptors passed by systemd using their names from environment variables."""
    listen_pid = os.environ.get('LISTEN_PID')
    listen_fds = os.environ.get('LISTEN_FDS')
    listen_fdnames = os.environ.get('LISTEN_FDNAMES')

    if not listen_pid or not listen_fds or not listen_fdnames:
        print("Fatal: Socket activation environment variables not set (LISTEN_PID, LISTEN_FDS, LISTEN_FDNAMES).")
        sys.exit(1)

    try:
        pid = int(listen_pid)
        num_fds = int(listen_fds)
    except ValueError:
        print("Fatal: Invalid LISTEN_PID or LISTEN_FDS value.")
        sys.exit(1)

    if pid != os.getpid():
        print(f"Fatal: LISTEN_PID ({pid}) does not match current PID ({os.getpid()}).")
        sys.exit(1)

    if num_fds < 1:
        print("Fatal: No file descriptors received (LISTEN_FDS is 0).")
        sys.exit(1)

    fd_names = listen_fdnames.split(':')
    if len(fd_names) != num_fds:
        print(f"Fatal: Mismatch between FD count ({num_fds}) and FD names count ({len(fd_names)}).")
        sys.exit(1)

    # File descriptors start at 3
    LISTEN_FDS_START = 3
    named_fds = {}
    for i, name in enumerate(fd_names):
        fd = LISTEN_FDS_START + i
        if name not in named_fds:
            named_fds[name] = []
        named_fds[name].append(fd)

    tcp_fds = named_fds.get('wireguard-pro-tcp', []) + named_fds.get('wireguard-pro-tcp6', [])
    udp_fds = named_fds.get('boringtun-udp', []) + named_fds.get('boringtun-udp6', [])

    if not tcp_fds:
        print("Fatal: Could not find any TCP sockets named 'wireguard-pro-tcp'.")
        sys.exit(1)

    if not udp_fds:
        print("Fatal: Could not find any UDP sockets named 'boringtun-udp'.")
        sys.exit(1)

    # We only expect one TCP socket for the web UI
    tcp_fd = tcp_fds[0]

    print(f"Identified TCP FD: {tcp_fd} for Gunicorn")
    print(f"Identified UDP FDs: {udp_fds} for BoringTun")

    return tcp_fd, udp_fds

def main():
    """Main bootstrap script to manage all services."""
    setup_secret_key()
    tcp_fd, udp_fds = get_named_socket_fds()

    # --- Start BoringTun ---
    print("Starting BoringTun...")
    boringtun_args = ['boringtun-cli', 'wg0', '--foreground', '--verbosity', 'debug']
    for fd in udp_fds:
        boringtun_args.extend(['--fd', str(fd)])
    
    boringtun_proc = subprocess.Popen(boringtun_args)

    # --- Configure WireGuard interface (after it's created by boringtun) ---
    print("Waiting for wg0 interface to be created")
    iface_path = '/sys/class/net/wg0'
    timeout = 5  # seconds
    start_time = time.time()
    while not os.path.exists(iface_path):
        if time.time() - start_time > timeout:
            print(f"Error: Timeout waiting for {iface_path} to appear")
            boringtun_proc.terminate()
            sys.exit(1)
        if boringtun_proc.poll() is not None:
            print(f"Error: boringtun process exited prematurely with code {boringtun_proc.returncode}.")
            sys.exit(1)
        time.sleep(0.1)

    print("wg0 interface created.")
    run_command(['wg', 'set', 'wg0', 'private-key', '/run/secrets/wg-privatekey'])
    run_command(['ip', 'address', 'add', '10.8.0.1/24', 'dev', 'wg0'])
    run_command(['ip', 'address', 'add', 'fd86:ea04:1111::1/64', 'dev', 'wg0'])
    run_command(['ip', 'link', 'set', 'mtu', '1420', 'up', 'dev', 'wg0'])

    # --- Start Gunicorn ---
    print("Starting Gunicorn")
    gunicorn_proc = subprocess.Popen([
        '/venv/bin/gunicorn', 'main:app',
        '--workers', '2',
        '--worker-class', 'uvicorn.workers.UvicornWorker',
        '--bind', f'fd://{tcp_fd}'
    ])

    # --- Apply nftables rules ---
    run_command(['nft', '-f', '/etc/nftables.conf'])

    # --- Start Caddy ---
    print("Starting Caddy...")
    caddy_proc = subprocess.Popen([
        'caddy', 'run', '--config', '/etc/caddy/Caddyfile', '--adapter', 'caddyfile'
    ])

    # --- Wait for any process to exit ---
    procs = [boringtun_proc, gunicorn_proc, caddy_proc]
    while True:
        for p in procs:
            if p.poll() is not None:
                print(f"Process {p.args[0]} exited with code {p.returncode}, Shutting down now") # type: ignore
                # Terminate other processes
                for other_p in procs:
                    if other_p.pid != p.pid:
                        other_p.terminate()
                sys.exit(1)
        time.sleep(1)

if __name__ == "__main__":
    main()
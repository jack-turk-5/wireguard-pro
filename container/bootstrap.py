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

import socket

def get_activated_sockets():
    """
    Inspects file descriptors passed by systemd and returns a tuple of
    (tcp_fds, udp_fds), leaving them open for other processes.
    """
    listen_pid = os.environ.get('LISTEN_PID')
    listen_fds = os.environ.get('LISTEN_FDS')

    if not listen_pid or not listen_fds:
        print("Info: No LISTEN_PID or LISTEN_FDS. No socket activation.")
        return [], []

    try:
        pid = int(listen_pid)
        num_fds = int(listen_fds)
    except (ValueError, TypeError):
        print(f"Fatal: Invalid LISTEN_PID ('{listen_pid}') or LISTEN_FDS ('{listen_fds}') value.")
        sys.exit(1)

    if pid != os.getpid():
        print(f"Fatal: LISTEN_PID ({pid}) does not match current PID ({os.getpid()}).")
        sys.exit(1)

    if num_fds < 1:
        return [], []

    LISTEN_FDS_START = 3
    tcp_fds = []
    udp_fds = []
    for fd_num in range(LISTEN_FDS_START, LISTEN_FDS_START + num_fds):
        try:
            sock = socket.fromfd(fd_num, socket.AF_UNSPEC, 0)
            if sock.type == socket.SOCK_STREAM:
                tcp_fds.append(fd_num)
            elif sock.type == socket.SOCK_DGRAM:
                udp_fds.append(fd_num)
            sock.detach() # Leave the original FD open
        except (OSError, socket.error):
            pass # Ignore non-socket FDs

    print(f"Identified TCP FDs: {tcp_fds}")
    print(f"Identified UDP FDs: {udp_fds}")
    return tcp_fds, udp_fds

def main():
    """Main bootstrap script to manage all services."""
    setup_secret_key()
    tcp_fds, udp_fds = get_activated_sockets()

    # --- Start BoringTun ---
    print("Starting BoringTun")
    boringtun_args = ['boringtun-cli', 'wg0', '--foreground', '--verbosity', 'debug', '--disable-drop-privileges']
    if not udp_fds:
        print("Warning: No UDP file descriptors found for BoringTun.")
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
        '--bind', 'unix:/run/gunicorn.sock'
    ])

    # --- Apply nftables rules ---
    run_command(['nft', '-f', '/etc/nftables.conf'])

    # --- Start Caddy ---
    print("Starting Caddy")
    caddy_env = os.environ.copy()
    if not tcp_fds:
        print("Warning: No TCP file descriptor found for Caddy.")
    else:
        # Pass the first available TCP socket FD to Caddy.
        caddy_env['CADDY_TCP_FD'] = str(tcp_fds[0])
        print(f"Passing TCP FD {tcp_fds[0]} to Caddy via CADDY_TCP_FD env var.")
    
    caddy_proc = subprocess.Popen(
        ['caddy', 'run', '--config', '/etc/caddy/Caddyfile', '--adapter', 'caddyfile'],
        env=caddy_env
    )

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
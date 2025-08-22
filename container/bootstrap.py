#!/usr/bin/env python3
import os
import subprocess
import sys
import socket
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

def get_socket_fds():
    """Get file descriptors passed by systemd and identify them."""
    listen_fds = int(os.environ.get('LISTEN_FDS', 0))
    if listen_fds < 2:
        print(f"Error: Expected 2 file descriptors from systemd, but got {listen_fds}. Exiting.")
        sys.exit(1)

    fd1, fd2 = 3, 4
    tcp_fd, udp_fd = None, None

    try:
        # Try to treat fd1 as TCP. If it works, we know the order.
        s = socket.fromfd(fd1, socket.AF_INET6, socket.SOCK_STREAM)
        s.close()
        tcp_fd, udp_fd = fd1, fd2
        print(f"FD {fd1} is TCP, FD {fd2} is UDP.")
    except OSError:
        # If it fails, the order must be swapped.
        tcp_fd, udp_fd = fd2, fd1
        print(f"FD {fd1} is not TCP, assuming FD {fd2} is TCP and FD {fd1} is UDP.")
    
    if tcp_fd is None or udp_fd is None:
        print(f"Fatal: Could not identify both a TCP and a UDP socket.")
        sys.exit(1)

    print(f"Identified TCP FD: {tcp_fd} for Gunicorn")
    print(f"Identified UDP FD: {udp_fd} for BoringTun")
    
    return tcp_fd, udp_fd

def main():
    """Main bootstrap script to manage all services."""
    setup_secret_key()
    tcp_fd, udp_fd = get_socket_fds()

    # --- Start BoringTun ---
    print("Starting BoringTun...")
    boringtun_proc = subprocess.Popen([
        'boringtun-cli', 'wg0',
        '--fd', str(udp_fd),
        '--foreground',
        '--verbosity', 'debug'
    ])

    # --- Configure WireGuard interface (after it's created by boringtun) ---
    print("Waiting for wg0 interface to be created...")
    iface_path = '/sys/class/net/wg0'
    timeout = 5  # seconds
    start_time = time.time()
    while not os.path.exists(iface_path):
        if time.time() - start_time > timeout:
            print(f"Error: Timeout waiting for {iface_path} to appear.")
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
    print("Starting Gunicorn...")
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
                print(f"Process {p.args[0]} exited with code {p.returncode}. Shutting down.")
                # Terminate other processes
                for other_p in procs:
                    if other_p.pid != p.pid:
                        other_p.terminate()
                sys.exit(1)
        time.sleep(1)

if __name__ == "__main__":
    main()

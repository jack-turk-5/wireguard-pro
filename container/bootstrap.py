#!/usr/bin/env python3
import os
import subprocess
from secrets import token_urlsafe
from shutil import which

def run_command(command, check=True):
    """Helper to run a command and log its output."""
    print(f"Running command: {' '.join(command)}")
    subprocess.run(command, check=check)

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

def setup_wireguard():
    """Configure and bring up the WireGuard interface."""
    conf_file = '/etc/wireguard/wg0.conf'
    secret_file = '/run/secrets/wg-privatekey'
    
    if not os.path.isfile(conf_file):
        os.makedirs(os.path.dirname(conf_file), exist_ok=True)
        if os.path.exists(secret_file):
            with open(secret_file) as f:
                private_key = f.read().strip()
        else:
            private_key = subprocess.check_output(['wg', 'genkey']).decode().strip()
        
        with open('/etc/wireguard/privatekey', 'w') as f:
            f.write(private_key)
            
        public_key = subprocess.check_output(['wg', 'pubkey'], input=private_key.encode()).decode().strip()
        
        config = [
            "[Interface]",
            f"PrivateKey = {private_key}",
            "Address = 10.8.0.1/24, fd86:ea04:1111::1/64",
            "ListenPort = 51820",
            "MTU = 1420"
        ]
        with open(conf_file, 'w') as f:
            f.write(f"{'/n'.join(config)}\n")

    # Bring up the interface
    run_command(['wg-quick', 'up', 'wg0'])

def main():
    """Main bootstrap script."""
    setup_secret_key()
    setup_wireguard()

    # Start Gunicorn with Uvicorn workers
    gunicorn_args = os.environ.get("GUNICORN_CMD_ARGS", "--workers 2 --worker-class uvicorn.workers.UvicornWorker --bind unix:/run/gunicorn.sock").split()
    subprocess.Popen(['/venv/bin/gunicorn', 'main:app', *gunicorn_args])

    # Apply nftables rules
    run_command(['nft', '-f', '/etc/nftables.conf'])

    # Exec into Caddy
    caddy_executable = which('caddy')
    if caddy_executable:
        os.execv(caddy_executable, [
            'caddy', 'run', '--config', '/etc/caddy/Caddyfile', '--adapter', 'caddyfile'
        ])
    else:
        print("Error: 'caddy' executable not found.")
        exit(1)

if __name__ == "__main__":
    main()
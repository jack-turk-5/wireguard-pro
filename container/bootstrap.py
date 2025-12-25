#!/usr/bin/env python3
import os
import subprocess
import stat
from secrets import token_urlsafe
from shutil import which


def run_command(command, check=True):
    """Helper to run a command and log its output."""
    print(f"Running command: {' '.join(command)}")
    subprocess.run(command, check=check)


def setup_secret_key():
    """Set up the application's secret key."""
    secret_file = "/data/app_secret"
    if "SECRET_KEY" not in os.environ:
        if os.path.exists(secret_file):
            with open(secret_file, "r") as f:
                os.environ["SECRET_KEY"] = f.read().strip()
        else:
            key = token_urlsafe(32)
            os.makedirs(os.path.dirname(secret_file), exist_ok=True)
            with open(secret_file, "w") as f:
                f.write(key)
            os.environ["SECRET_KEY"] = key


def get_interface_info():
    """Get info from environmnet for wg0 interface"""
    port = os.environ.get("WG_PORT")
    if not port:
        raise Exception("Missing WG_PORT")
    return {
        "ipv4": os.environ.get("WG_IPV4_BASE_ADDRESS", "10.8.0.1"),
        "ipv6": os.environ.get("WG_IPV6_BASE_ADDRESS", "fd86:ea04:1111::1"),
        "port": port.strip("'\""),
    }


def setup_wireguard():
    """Configure and bring up the WireGuard interface."""
    conf_file = "/etc/wireguard/wg0.conf"
    private_key_file = "/etc/wireguard/privatekey"
    secret_file = "/run/secrets/wg-privatekey"

    # Ensure private key exists
    if os.path.exists(private_key_file):
        with open(private_key_file) as f:
            private_key = f.read().strip()
    elif os.path.exists(secret_file):
        with open(secret_file) as f:
            private_key = f.read().strip()
        with open(private_key_file, "w") as f:
            f.write(private_key)
    else:
        private_key = subprocess.check_output(["wg", "genkey"]).decode().strip()
        with open(private_key_file, "w") as f:
            f.write(private_key)

    interface_info = get_interface_info()

    # Check for port changes and warn the user
    if os.path.exists(conf_file):
        with open(conf_file, "r") as f:
            for line in f:
                if line.strip().startswith("ListenPort"):
                    existing_port = line.split("=")[1].strip()
                    if existing_port != interface_info["port"]:
                        print("!!! WARNING: Port has changed! You will need to update your clients. !!!")
                        print(f"!!! Old port: {existing_port} -> New port: {interface_info['port']} !!!")
                    break

    config = [
        "[Interface]",
        f"PrivateKey = {private_key}",
        f"Address = {interface_info['ipv4']}, {interface_info['ipv6']}",
        f"ListenPort = {interface_info['port']}",
        "MTU = 1420",
    ]

    os.makedirs(os.path.dirname(conf_file), exist_ok=True)
    with open(conf_file, "w", newline="\n") as f:
        f.write("\n".join(config) + "\n")

    # Check file permissions for config
    os.chmod(conf_file, stat.S_IRUSR | stat.S_IWUSR)
    # Bring up the interface
    run_command(["wg-quick", "up", "wg0"])


def main():
    """Main bootstrap script."""
    setup_secret_key()
    setup_wireguard()

    # Start Gunicorn with Uvicorn workers
    gunicorn_args = os.environ.get(
        "GUNICORN_CMD_ARGS",
        "--workers 2 --worker-class uvicorn.workers.UvicornWorker --bind unix:/run/gunicorn.sock",
    ).split()
    subprocess.Popen(["/venv/bin/gunicorn", "main:app", *gunicorn_args])

    # Apply nftables rules
    run_command(["nft", "-f", "/etc/nftables.conf"])

    # Exec into Caddy
    caddy_executable = which("caddy")
    if caddy_executable:
        os.execv(
            caddy_executable,
            [
                "caddy",
                "run",
                "--config",
                "/etc/caddy/Caddyfile",
                "--adapter",
                "caddyfile",
            ],
        )
    else:
        print("Error: 'caddy' executable not found.")
        exit(1)


if __name__ == "__main__":
    main()

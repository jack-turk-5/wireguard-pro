import os
import subprocess

wg_conf = "/etc/wireguard/wg0.conf"
secret_path = "/run/secrets/wg_privatekey"

if not os.path.isfile(wg_conf):
    print("[Bootstrap] No wg0.conf found, generating default WireGuard config...")
    os.makedirs("/etc/wireguard", exist_ok=True)

    if os.path.exists(secret_path):
        with open(secret_path) as f:
            private_key = f.read().strip()
        print("[Bootstrap] Loaded WireGuard private key from Podman secret.")
    else:
        private_key = subprocess.check_output(["wg", "genkey"]).decode().strip()
        print("[Bootstrap] No secret found, generated new random private key.")

    with open("/etc/wireguard/privatekey", "w") as f:
        f.write(private_key)
    with open("/etc/wireguard/publickey", "w") as f:
        pubkey = subprocess.check_output(["wg", "pubkey"], input=private_key.encode()).decode().strip()
        f.write(pubkey)
    with open(wg_conf, "w") as f:
        f.write(f"""[Interface]
PrivateKey = {private_key}
Address = 10.8.0.1/24, fd86:ea04:1111::1/64
ListenPort = 51820
SaveConfig = true
""")
else:
    print("[Bootstrap] wg0.conf already exists.")

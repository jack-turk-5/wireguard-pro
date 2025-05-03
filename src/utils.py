from db import get_all_peers
from subprocess import check_output, run, PIPE


# 1. Generate a private/public keypair
def generate_keypair():
    private_key = check_output(["wg", "genkey"]).decode().strip()
    res = run(
        args=["wg", "pubkey"],
        input=private_key,
        capture_output=True,
        check=True,
        text=True
    )
    public_key = res.stdout.strip()
    return private_key, public_key


# 2. Derive server public key from stored private key
def get_server_pubkey():
    priv = open('/etc/wireguard/privatekey').read().strip()
    proc = run(
        ['wg', 'pubkey'],
        input=priv.encode(),
        stdout=PIPE,
        check=True
    )
    return proc.stdout.decode().strip()


# 3. Allocate the next free IPv4/IPv6 addresses
def next_available_ip():
    peers = get_all_peers()
    used_v4 = {p['ipv4_address'] for p in peers}
    used_v6 = {p['ipv6_address'] for p in peers}

    for i in range(2, 255):
        candidate = f"10.8.0.{i}"
        if candidate not in used_v4:
            ipv4 = candidate
            break
    else:
        raise RuntimeError("No free IPv4 addresses left in 10.8.0.0/24")

    for suffix in range(0x100, 0x10000):
        candidate6 = f"fd86:ea04:1111::{suffix:x}"
        if candidate6 not in used_v6:
            ipv6 = candidate6
            break
    else:
        raise RuntimeError("No free IPv6 addresses left in fd86:ea04:1111::/64")

    return ipv4, ipv6


# 4. Append a peer to the wg0.conf file
def append_peer_to_wgconf(public_key, ipv4, ipv6):
    with open("/etc/wireguard/wg0.conf", "a") as f:
        lines = [
            "[Peer]",
            f"PublicKey = {public_key}",
            f"AllowedIPs = {ipv4}/32, {ipv6}/128"
        ]
        f.write("\n".join(lines) + "\n")


# 5. Reload peers dynamically (strip + syncconf)
def reload_wireguard():
    strip = run(["wg-quick", "strip", "wg0"], stdout=PIPE, check=True, text=True)
    with open("/run/wg0.peers.conf", "w") as f:
        f.write(strip.stdout)

    run(["wg", "syncconf", "wg0", "/run/wg0.peers.conf"], check=True)
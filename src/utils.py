# utils.py

from db import get_all_peers
from subprocess import check_output, run

# 1. keypair gen stays the same
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

# 2. dynamic IP allocation
def next_available_ip():
    """
    Scans the DB for used IPs and picks the next free ones
    on 10.8.0.0/24 and fd86:ea04:1111::/64
    """
    peers = get_all_peers()
    used_v4 = {p['ipv4_address'] for p in peers}
    used_v6 = {p['ipv6_address'] for p in peers}

    # IPv4: 10.8.0.2 → 10.8.0.254
    for i in range(2, 255):
        candidate = f"10.8.0.{i}"
        if candidate not in used_v4:
            ipv4 = candidate
            break
    else:
        raise RuntimeError("No free IPv4 addresses left in 10.8.0.0/24")

    # IPv6: fd86:ea04:1111::100 → fd86:ea04:1111::ffff
    for suffix in range(0x100, 0x10000):
        candidate6 = f"fd86:ea04:1111::{suffix:x}"
        if candidate6 not in used_v6:
            ipv6 = candidate6
            break
    else:
        raise RuntimeError("No free IPv6 addresses left in fd86:ea04:1111::/64")

    return ipv4, ipv6

def append_peer_to_wgconf(public_key, ipv4, ipv6):
    with open("/etc/wireguard/wg0.conf", "a") as f:
        f.write(f"""
[Peer]
PublicKey = {public_key}
AllowedIPs = {ipv4}/32, {ipv6}/128
""")

def reload_wireguard():
    run(["wg", "syncconf", "wg0", "/etc/wireguard/wg0.conf"])
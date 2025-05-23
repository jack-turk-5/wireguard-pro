from db import get_all_peers
from subprocess import check_output, run, PIPE


WG_PATH = "/etc/wireguard/wg0.conf"

def generate_keypair():
    """
    Generate a private/public keypair
    """
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

def get_server_pubkey():
    """
    Derive server public key from stored private key
    """
    priv = open('/etc/wireguard/privatekey').read().strip()
    proc = run(
        ['wg', 'pubkey'],
        input=priv.encode(),
        stdout=PIPE,
        check=True
    )
    return proc.stdout.decode().strip()

def next_available_ip():
    """
    Allocate the next free IPv4/IPv6 addresses
    """
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

def append_peer_to_wgconf(public_key, ipv4, ipv6):
    """
    Append a peer to the wg0.conf file
    """
    with open(WG_PATH, "a") as f:
        lines = [
            "[Peer]",
            f"PublicKey = {public_key}",
            f"AllowedIPs = {ipv4}/32, {ipv6}/128"
        ]
        f.write("\n".join(lines) + "\n")

def reload_wireguard():
    """
    Reload peers dynamically (strip + syncconf)
    """
    strip = run(["wg-quick", "strip", "wg0"], stdout=PIPE, check=True, text=True)
    with open("/run/wg0.peers.conf", "w") as f:
        f.write(strip.stdout)

    run(["wg", "syncconf", "wg0", "/run/wg0.peers.conf"], check=True)

def remake_peers_file():
    # 1) read existing file up to—but not including—the first [Peer]
    with open(WG_PATH, "r") as f:
        lines = f.read().splitlines()
    interface_section = []
    for line in lines:
        if line.strip() == "[Peer]":
            break
        interface_section.append(line)

    # 2) overwrite disk config with just the interface
    with open(WG_PATH, "w") as f:
        f.write("\n".join(interface_section) + "\n")

    # 3) re-append every peer from the DB
    for p in get_all_peers():
        append_peer_to_wgconf(p["public_key"], p["ipv4_address"], p["ipv6_address"])

    # 4) push into the running interface
    reload_wireguard()

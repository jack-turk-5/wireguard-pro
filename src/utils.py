import re
from db import get_all_peers
from subprocess import check_output, run, PIPE

WG_PATH = "/etc/wireguard/wg0.conf"

def generate_keypair():
    """Generate a private/public keypair."""
    private_key = check_output(["wg", "genkey"]).decode().strip()
    public_key = check_output(["wg", "pubkey"], input=private_key.encode()).decode().strip()
    return private_key, public_key

def get_server_pubkey():
    """Derive server public key from stored private key."""
    with open('/etc/wireguard/privatekey') as f:
        private_key = f.read().strip()
    return check_output(['wg', 'pubkey'], input=private_key.encode()).decode().strip()

def next_available_ip():
    """Allocate the next free IPv4/IPv6 addresses."""
    peers = get_all_peers()
    used_v4 = {p['ipv4_address'] for p in peers}
    used_v6 = {p['ipv6_address'] for p in peers}

    # Find next available IPv4
    for i in range(2, 255):
        candidate_v4 = f"10.8.0.{i}"
        if candidate_v4 not in used_v4:
            ipv4 = candidate_v4
            break
    else:
        raise RuntimeError("No free IPv4 addresses left in 10.8.0.0/24")

    # Find next available IPv6
    for i in range(2, 65535):
        candidate_v6 = f"fd86:ea04:1111::{i:x}"
        if candidate_v6 not in used_v6:
            ipv6 = candidate_v6
            break
    else:
        raise RuntimeError("No free IPv6 addresses left in fd86:ea04:1111::/64")

    return ipv4, ipv6

def append_peer_to_wgconf(public_key, ipv4, ipv6):
    """Append a peer to the wg0.conf file."""
    with open(WG_PATH, "a") as f:
        f.write(f"\n[Peer]\nPublicKey = {public_key}\nAllowedIPs = {ipv4}/32, {ipv6}/128\n")

def remove_peer_from_wgconf(public_key: str):
    """
    Removes a peer's [Peer] block from the wg0.conf file.
    """
    with open(WG_PATH, "r") as f:
        lines = f.readlines()

    # Find the start of the peer block
    try:
        start_index = lines.index(f"PublicKey = {public_key}\n") - 1
        # Ensure we found a [Peer] block
        if lines[start_index].strip() != "[Peer]":
            return
    except ValueError:
        return # Peer not found in file

    # Find the end of the block (next [Peer] or end of file)
    end_index = start_index + 1
    while end_index < len(lines) and lines[end_index].strip() != "[Peer]":
        end_index += 1

    # Remove the lines for the peer
    del lines[start_index:end_index]

    # Write the file back
    with open(WG_PATH, "w") as f:
        f.writelines(lines)
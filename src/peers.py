# peers.py

from subprocess import check_output, run
from datetime import datetime, timezone, timedelta

from db import add_peer_db, remove_peer_db, get_all_peers
from utils import generate_keypair, next_available_ip, append_peer_to_wgconf, reload_wireguard

WG_PATH = "/etc/wireguard/wg0.conf"

def create_peer(days_valid=7):
    # 1) gen keys
    priv, pub = generate_keypair()

    # 2) grab the next free IPs
    ipv4, ipv6 = next_available_ip()

    # 3) compute expiration (space‑separated format)
    expires = datetime.now(timezone.utc) + timedelta(days=days_valid)
    expires_str = expires.strftime("%Y-%m-%d %H:%M:%S")

    # 4) **only one** DB insert
    add_peer_db(pub, priv, ipv4, ipv6, expires_str)

    # 5) wireguard config + reload
    append_peer_to_wgconf(pub, ipv4, ipv6)
    reload_wireguard()

    # 6) return what the front‑end needs for QR + display
    return {
        "private_key": priv,
        "public_key": pub,
        "ipv4_address": ipv4,
        "ipv6_address": ipv6,
        "expires_at": expires_str
    }

def delete_peer(public_key):
    success = remove_peer_db(public_key)
    if not success:
        return False

    # Read the file
    lines = open(WG_PATH).read().splitlines()
    new_lines = []
    skip = False
    for line in lines:
        if public_key in line and line.startswith("PublicKey"):
            skip = True       # start skipping at [Peer] block
            continue
        if skip:
            # after dropping the PublicKey line, skip until blank line
            if line.strip() == "":
                skip = False
            continue
        new_lines.append(line)

    # Write back
    with open(WG_PATH, "w") as f:
        f.write("\n".join(new_lines) + "\n")

    # Now remove from live interface + reload
    run(["wg", "set", "wg0", "peer", public_key, "remove"], check=True)
    run(["wg", "setconf", "wg0", WG_PATH], check=True)

    return True

def list_peers():
    return get_all_peers()

def peer_stats():
    output = check_output(["wg", "show", "wg0", "dump"]).decode()
    lines = output.strip().split('\n')[1:]
    stats = []
    for line in lines:
        f = line.split('\t')
        if len(f) < 8: continue
        pub, _, _, _, last_hs, rx, tx, keepalive, *_ = f
        stats.append({
            "public_key": pub,
            "last_handshake_time": last_hs,
            "rx_bytes": rx,
            "tx_bytes": tx,
            "persistent_keepalive": keepalive,
        })
    return stats
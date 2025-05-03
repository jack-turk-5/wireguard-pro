# peers.py

from subprocess import check_output, run
from datetime import datetime, timezone, timedelta

from db import add_peer_db, remove_peer_db, get_all_peers
from utils import generate_keypair, next_available_ip, append_peer_to_wgconf, reload_wireguard

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
    if success:
        # you can keep using sed or switch to a Python‑based removal
        run(["sed", "-i", f"/{public_key}/,+2d", "/etc/wireguard/wg0.conf"])
        reload_wireguard()
    return success

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
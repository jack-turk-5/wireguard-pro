from subprocess import check_output, run
from datetime import datetime, timezone, timedelta

from db import add_peer_db, remove_peer_db, get_all_peers
from utils import generate_keypair, next_available_ip, append_peer_to_wgconf, remake_peers_file

# Path to the on-disk WireGuard config file
WG_PATH = "/etc/wireguard/wg0.conf"

def create_peer(days_valid=7):
    """
    Generate a new peer, store it in the DB, append to disk config,
    and inject into the running WireGuard interface
    """
    # Generate keypair
    priv, pub = generate_keypair()

    # Allocate next free IPv4/IPv6
    ipv4, ipv6 = next_available_ip()

    # Compute expiration timestamp
    expires = datetime.now(timezone.utc) + timedelta(days=days_valid)
    expires_str = expires.strftime("%Y-%m-%d %H:%M:%S")

    # Persist in database
    add_peer_db(pub, priv, ipv4, ipv6, expires_str)

    # Append to the on-disk WireGuard config
    append_peer_to_wgconf(pub, ipv4, ipv6)

    # Inject into the running interface without full reload
    run([
        "wg", "set", "wg0",
        "peer", pub,
        "allowed-ips", f"{ipv4}/32,{ipv6}/128"
    ], check=True)

    # Return details for frontend
    return {
        "private_key": priv,
        "public_key": pub,
        "ipv4_address": ipv4,
        "ipv6_address": ipv6,
        "expires_at": expires_str
    }

def delete_peer(public_key):
    """
    Remove a peer by public key: delete from DB, remove stanza on disk,
    and remove from the running interface
    """
    success = remove_peer_db(public_key)
    if not success:
        return False
    else:
        remake_peers_file()
        return True

def list_peers():
    """
    Return all stored peers from the database
    """
    return get_all_peers()

def peer_stats():
    """
    Return live WireGuard peer stats
    """
    output = check_output(["wg", "show", "wg0", "dump"]).decode()
    lines = output.strip().split("\n")[1:]
    stats = []
    for line in lines:
        fields = line.split("\t")
        if len(fields) < 8:
            continue
        pub, _, _, _, last_hs, rx, tx, keepalive, *_ = fields
        stats.append({
            "public_key": pub,
            "last_handshake_time": last_hs,
            "rx_bytes": rx,
            "tx_bytes": tx,
            "persistent_keepalive": keepalive,
        })
    return stats
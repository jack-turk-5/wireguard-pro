from subprocess import run, check_output
from datetime import datetime, timezone, timedelta
from db import add_peer_db, remove_peer_db, get_all_peers
from utils import generate_keypair, next_available_ip, append_peer_to_wgconf, reload_wireguard

def create_peer(days_valid=7):
    private_key, public_key = generate_keypair()
    ipv4, ipv6 = next_available_ip()
    expires = datetime.now(timezone.utc) + timedelta(days=days_valid)
    expires_str = expires.strftime("%Y-%m-%d %H:%M:%S")  # space, no T
    add_peer_db(public_key, private_key, ipv4, ipv6, expires_str)

    add_peer_db(public_key, private_key, ipv4, ipv6, expires.isoformat())
    append_peer_to_wgconf(public_key, ipv4, ipv6)
    reload_wireguard()

    return {
        "private_key": private_key,
        "public_key": public_key,
        "ipv4": ipv4,
        "ipv6": ipv6,
        "expires_at": expires.isoformat()
    }

def delete_peer(public_key):
    success = remove_peer_db(public_key)
    if success:
        run(["sed", "-i", f"/{public_key}/,+2d", "/etc/wireguard/wg0.conf"])
        reload_wireguard()
    return success

def list_peers():
    return get_all_peers()

def peer_stats():
    output = check_output(["wg", "show", "wg0", "dump"]).decode()
    lines = output.strip().split('\n')
    stats = []

    for line in lines[1:]:
        fields = line.split('\t')
        if len(fields) < 5:
            continue
        public_key, _endpoint, _allowed_ips, _persistent_keepalive_flag, last_hs, rx, tx, keepalive, *_ = fields

        stats.append({
            "public_key": public_key,
            "last_handshake_time": last_hs,
            "rx_bytes": rx,
            "tx_bytes": tx,
            "persistent_keepalive": keepalive,
        })

    return stats
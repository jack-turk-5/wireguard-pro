import subprocess
import datetime
from db import add_peer_db, remove_peer_db, get_all_peers
from utils import generate_keypair, next_available_ip, append_peer_to_wgconf, reload_wireguard

def create_peer(days_valid=7):
    private_key, public_key = generate_keypair()
    ipv4, ipv6 = next_available_ip()
    expires = datetime.datetime.utcnow() + datetime.timedelta(days=days_valid)

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
        subprocess.run(["sed", "-i", f"/{public_key}/,+2d", "/etc/wireguard/wg0.conf"])
        reload_wireguard()
    return success

def list_peers():
    return get_all_peers()

def peer_stats():
    output = subprocess.check_output(["wg", "show", "wg0", "dump"]).decode()
    lines = output.strip().split('\n')
    stats = []

    for line in lines[1:]:
        fields = line.split('\t')
        if len(fields) < 5:
            continue
        stats.append({
            "public_key": fields[0],
            "last_handshake_time": int(fields[4]),
            "rx_bytes": int(fields[5]),
            "tx_bytes": int(fields[6]),
            "persistent_keepalive": fields[7],
        })

    return stats
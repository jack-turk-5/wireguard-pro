from subprocess import check_output, run
from datetime import datetime, timezone, timedelta

from db import add_peer_db, remove_peer_db, get_all_peers
from utils import generate_keypair, next_available_ip, append_peer_to_wgconf

WG_PATH = "/etc/wireguard/wg0.conf"


def create_peer(days_valid=7):
    # 1) Generate keypair
    priv, pub = generate_keypair()

    # 2) Allocate next free IPs
    ipv4, ipv6 = next_available_ip()

    # 3) Compute expiration timestamp
    expires = datetime.now(timezone.utc) + timedelta(days=days_valid)
    expires_str = expires.strftime("%Y-%m-%d %H:%M:%S")

    # 4) Store in database
    add_peer_db(pub, priv, ipv4, ipv6, expires_str)

    # 5) Append to on-disk config
    append_peer_to_wgconf(pub, ipv4, ipv6)

    # 6) Inject into live interface (no full reload)
    run([
        "wg", "set", "wg0",
        "peer", pub,
        "allowed-ips", f"{ipv4}/32,{ipv6}/128"
    ], check=True)

    # 7) Return for UI
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

    # 1) Remove from on-disk config file
    lines = open(WG_PATH).read().splitlines()
    new_lines = []
    skip = False
    for line in lines:
        if skip:
            # stop skipping at blank line
            if line.strip() == "":
                skip = False
            continue
        if line.startswith("PublicKey") and public_key in line:
            skip = True
            continue
        new_lines.append(line)

    with open(WG_PATH, "w") as f:
        f.write("\n".join(new_lines) + "\n")

    # 2) Remove peer from running interface
    run(["wg", "set", "wg0", "peer", public_key, "remove"], check=True)
    return True


def list_peers():
    return get_all_peers()


def peer_stats():
    output = check_output(["wg", "show", "wg0", "dump"]).decode()
    lines = output.strip().split('\n')[1:]
    stats = []
    for line in lines:
        f = line.split('\t')
        if len(f) < 8:
            continue
        pub, _, _, _, last_hs, rx, tx, keepalive, *_ = f
        stats.append({
            "public_key": pub,
            "last_handshake_time": last_hs,
            "rx_bytes": rx,
            "tx_bytes": tx,
            "persistent_keepalive": keepalive,
        })
    return stats
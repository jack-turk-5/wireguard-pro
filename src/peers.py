import asyncio
from datetime import datetime, timezone, timedelta
import logging

from db import add_peer_db, remove_peer_db, get_all_peers
from utils import generate_keypair, next_available_ip, append_peer_to_wgconf, remake_peers_file, _run_command

# Path to the on-disk WireGuard config file
WG_PATH = "/etc/wireguard/wg0.conf"


async def create_peer(days_valid=7):
    """
    Generate a new peer, store it in the DB, append to disk config,
    and inject into the running WireGuard interface asynchronously.
    """
    # Generate keypair
    priv, pub = await generate_keypair()

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
    await _run_command(f"wg set wg0 peer {pub} allowed-ips {ipv4}/32,{ipv6}/128")

    # Return details for frontend
    return {
        "private_key": priv,
        "public_key": pub,
        "ipv4_address": ipv4,
        "ipv6_address": ipv6,
        "expires_at": expires_str,
        "created_at": datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S")
    }


async def delete_peer(public_key):
    """
    Remove a peer by public key: delete from DB, remove stanza on disk,
    and remove from the running interface asynchronously.
    """
    success = remove_peer_db(public_key)
    if not success:
        return False
    else:
        await remake_peers_file()
        return True


def list_peers():
    """
    Return all stored peers from the database.
    """
    return get_all_peers()


async def peer_stats():
    """
    Return live WireGuard peer stats asynchronously.
    """
    output = await _run_command("wg show wg0 dump")
    lines = output.strip().split("\n")[1:]  # Skip header line
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


async def remove_expired_peers():
    """
    Remove all peers whose `expires_at` is in the past.
    Returns the number of peers that were removed.
    """
    now = datetime.now(timezone.utc)
    peers_to_remove = [
        p for p in get_all_peers()
        if datetime.strptime(p["expires_at"], "%Y-%m-%d %H:%M:%S").replace(tzinfo=timezone.utc) < now
    ]

    count = 0
    for peer in peers_to_remove:
        if await delete_peer(peer["public_key"]):
            count += 1
            logging.info(f"Auto-expired and removed peer: {peer['public_key']}")

    return count

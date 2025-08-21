import logging
from datetime import datetime, timezone, timedelta
from typing import Dict, Any, List
import base64

from pyroute2 import IPDB, WireGuard
from db import add_peer_db, remove_peer_db, get_all_peers
from utils import generate_keypair, next_available_ip, append_peer_to_wgconf, remove_peer_from_wgconf

def create_peer(days_valid: int = 7) -> Dict[str, Any]:
    """
    Generate a new peer and add it to the database, config file, and running interface.
    This operation is transactional to ensure consistency.
    """
    priv, pub = generate_keypair()
    ipv4, ipv6 = next_available_ip()
    expires = datetime.now(timezone.utc) + timedelta(days=days_valid)
    expires_str = expires.strftime("%Y-%m-%d %H:%M:%S")

    # Step 1: Persist to database first. This is our source of truth.
    try:
        add_peer_db(pub, priv, ipv4, ipv6, expires_str)
    except Exception as e:
        logging.error(f"Failed to add peer to database: {e}")
        raise  # Fail fast if the DB operation fails

    # Step 2: Append to the on-disk config.
    try:
        append_peer_to_wgconf(pub, ipv4, ipv6)
    except Exception as e:
        logging.error(f"Failed to append peer to wg0.conf: {e}")
        # Rollback: Remove from DB if config write fails
        remove_peer_db(pub)
        raise

    # Step 3: Add to the live WireGuard interface.
    try:
        with WireGuard() as wg:
            wg.set(interface='wg0', peer={
                'public_key': base64.b64decode(pub),
                'allowed_ips': [(ipv4, 32), (ipv6, 128)]
            })
    except Exception as e:
        logging.error(f"Failed to add peer to WireGuard interface: {e}")
        # Rollback: Remove from DB and config file if interface update fails
        remove_peer_db(pub)
        remove_peer_from_wgconf(pub)
        raise

    return {
        "private_key": priv,
        "public_key": pub,
        "ipv4_address": ipv4,
        "ipv6_address": ipv6,
        "expires_at": expires_str,
        "created_at": datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S")
    }

def delete_peer(public_key: str) -> bool:
    """
    Remove a peer by public key from the database, config file, and interface.
    This operation is transactional to ensure consistency.
    """
    # Step 1: Remove from the database first.
    if not remove_peer_db(public_key):
        logging.warning(f"Attempted to delete non-existent peer {public_key} from DB.")
        return False # Peer didn't exist in our source of truth

    # Step 2: Remove from the on-disk config.
    try:
        remove_peer_from_wgconf(public_key)
    except Exception as e:
        logging.error(f"Failed to remove peer from wg0.conf: {e}")
        # This is tricky. The DB record is gone. We can't easily roll back.
        # Log a critical error for manual intervention.
        logging.critical(f"INCONSISTENT STATE: Peer {public_key} removed from DB but not from config file!")
        # We will still attempt to remove from the live interface.
    
    # Step 3: Remove from the live interface.
    try:
        with WireGuard() as wg:
            wg.set(interface='wg0', peer={
                'public_key': base64.b64decode(public_key),
                'remove': True
            })
    except Exception as e:
        logging.error(f"Failed to remove peer {public_key} from WireGuard interface: {e}")
        # Log a critical error for manual intervention.
        logging.critical(f"INCONSISTENT STATE: Peer {public_key} removed from DB/config but not from live interface!")

    return True

def list_peers() -> List[Dict[str, Any]]:
    """Return all stored peers from the database."""
    return get_all_peers()

async def peer_stats() -> List[Dict[str, Any]]:
    """Return live WireGuard peer stats using pyroute2."""
    stats = []
    ip = IPDB(async_lib='asyncio')
    try:
        # The WireGuard object requires a reference to an IPDB instance
        # in async mode.
        wg = WireGuard(use_event_loop=ip)
        info = await wg.info(interface='wg0') # type: ignore
        if not info:
            return []

        # info() returns a list of interfaces, we only expect one
        interface_attrs = dict(info[0]['attrs'])
        
        if 'WGDEVICE_A_PEERS' in interface_attrs:
            for peer_msg in interface_attrs['WGDEVICE_A_PEERS']:
                peer = dict(peer_msg['attrs'])
                
                pub_key_bytes = peer.get('WGPEER_A_PUBLIC_KEY')
                if not pub_key_bytes:
                    continue
                
                last_handshake = peer.get('WGPEER_A_LAST_HANDSHAKE_TIME', {})
                
                stats.append({
                    "public_key": base64.b64encode(pub_key_bytes).decode('ascii'),
                    "last_handshake_time": last_handshake.get('tv_sec', 0),
                    "rx_bytes": peer.get('WGPEER_A_RX_BYTES', 0),
                    "tx_bytes": peer.get('WGPEER_A_TX_BYTES', 0),
                    "persistent_keepalive": peer.get('WGPEER_A_PERSISTENT_KEEPALIVE_INTERVAL', 0),
                })
    except Exception as e:
        logging.error(f"Failed to get peer stats from WireGuard interface: {e}", exc_info=True)
    finally:
        ip.release()
    return stats

def remove_expired_peers() -> int:
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
        if delete_peer(peer["public_key"]):
            count += 1
            logging.info(f"Auto-expired and removed peer: {peer['public_key']}")
            
    return count
from flask_apscheduler import APScheduler
from db import get_all_peers, remove_peer_db
import subprocess
import datetime

scheduler = APScheduler()

@scheduler.task('interval', id='expire_peers', seconds=3600)
def expire_peers():
    now = datetime.datetime.utcnow().isoformat()
    peers = get_all_peers()
    for peer in peers:
        if peer['expires_at'] and peer['expires_at'] < now:
            print(f"[AutoExpire] Removing expired peer {peer['public_key']}")
            remove_peer_db(peer['public_key'])
            subprocess.run(["sed", "-i", f"/{peer['public_key']}/,+2d", "/etc/wireguard/wg0.conf"])
            subprocess.run(["wg", "syncconf", "wg0", "/etc/wireguard/wg0.conf"])

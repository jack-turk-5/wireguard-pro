from flask_apscheduler import APScheduler
from peers import list_peers, delete_peer
from datetime import datetime, timezone


scheduler = APScheduler()

@scheduler.task('interval', id='expire_peers', seconds=3600)
def expire_peers():
    now = datetime.now(timezone.utc).isoformat()
    peers = list_peers()
    for peer in peers:
        if peer['expires_at'] and peer['expires_at'] < now:
            print(f"[AutoExpire] Removing expired peer {peer['public_key']}")
            delete_peer(peer['public_key'])
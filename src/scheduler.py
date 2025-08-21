from apscheduler.schedulers.background import BackgroundScheduler
from peers import remove_expired_peers
import logging

# Use a standard BackgroundScheduler
scheduler = BackgroundScheduler()

def expire_peers_job():
    """Scheduled job to remove expired peers."""
    logging.info("Scheduler: Running job to remove expired peers...")
    try:
        removed_count = remove_expired_peers()
        if removed_count > 0:
            logging.info(f"Scheduler: Successfully removed {removed_count} expired peers.")
        else:
            logging.info("Scheduler: No expired peers found.")
    except Exception as e:
        logging.error(f"Scheduler: Error during expired peer removal: {e}")

# Add the job to the scheduler instance
scheduler.add_job(expire_peers_job, 'interval', id='expire_peers', hours=1)

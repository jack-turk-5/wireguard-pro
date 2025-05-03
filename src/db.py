# db.py
import os
import sqlite3

# single source of truth for your SQLite file
DB_FILE = "/data/peers.db"

def _ensure_dir():
    d = os.path.dirname(DB_FILE)
    os.makedirs(d, exist_ok=True)

def db_conn():
    _ensure_dir()
    conn = sqlite3.connect(DB_FILE)
    conn.row_factory = sqlite3.Row
    return conn

def init_db():
    """
    Must be run once at startup (after /data is mounted):
    creates the peers table (with private_key) if it doesn't exist.
    """
    print(f"[init_db] opening {DB_FILE}")
    conn = db_conn()
    c = conn.cursor()
    c.execute("""
      CREATE TABLE IF NOT EXISTS peers (
        public_key   TEXT PRIMARY KEY,
        private_key  TEXT,
        ipv4_address TEXT,
        ipv6_address TEXT,
        created_at   TEXT    DEFAULT (datetime('now')),
        expires_at   TIMESTAMP
      );
    """)
    conn.commit()
    # sanity check
    c.execute("SELECT name FROM sqlite_master WHERE type='table' AND name='peers';")
    if not c.fetchone():
        raise RuntimeError("init_db failed to create peers table")
    print("[init_db] peers table is ready")
    c.close()
    conn.close()

def add_peer_db(pub, priv, ipv4, ipv6, expires):
    with db_conn() as conn:
        conn.execute("""
          INSERT INTO peers
            (public_key, private_key, ipv4_address, ipv6_address, expires_at)
          VALUES (?, ?, ?, ?, ?)
        """, (pub, priv, ipv4, ipv6, expires))

def remove_peer_db(pub):
    with db_conn() as conn:
        cur = conn.execute("DELETE FROM peers WHERE public_key = ?", (pub,))
        return cur.rowcount > 0

def get_all_peers():
    with db_conn() as conn:
        cur = conn.execute("""
          SELECT public_key,
                 private_key,
                 ipv4_address,
                 ipv6_address,
                 created_at,
                 expires_at
            FROM peers
        """)
        rows = cur.fetchall()
    return [dict(row) for row in rows]
from os import makedirs, path
from sqlite3 import connect, Row, IntegrityError
from bcrypt import checkpw, hashpw, gensalt

DB_FILE = "/data/peers.db"


def _ensure_dir():
    makedirs(path.dirname(DB_FILE), exist_ok=True)

def db_conn():
    _ensure_dir()
    conn = connect(DB_FILE)
    conn.row_factory = Row
    return conn

def init_db():
    conn = db_conn()
    c = conn.cursor()
    c.execute("""
      CREATE TABLE IF NOT EXISTS peers (
        public_key TEXT PRIMARY KEY,
        private_key TEXT,
        ipv4_address TEXT,
        ipv6_address TEXT,
        created_at TEXT DEFAULT (datetime('now')),
        expires_at TEXT
      )
    """)
    c.execute("""
      CREATE TABLE IF NOT EXISTS users (
        username TEXT PRIMARY KEY,
        password_hash BLOB NOT NULL,
        created_at TEXT DEFAULT (datetime('now'))
      )
    """)
    conn.commit()
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

def add_user_db(username: str, password: str) -> bool:
    """
    Hashes `password` with bcrypt and inserts a new user.
    Returns True on success, False if username already exists.
    """
    pwd_hash = hashpw(password.encode('utf-8'), gensalt())
    try:
        with db_conn() as conn:
            conn.execute(
                "INSERT INTO users (username, password_hash) VALUES (?, ?)",
                (username, pwd_hash)
            )
        return True
    except IntegrityError:
        # username already exists
        return False

def verify_user_db(username: str, password: str) -> bool:
    """
    Fetches the stored hash for `username` and verifies `password`.
    Returns True if credentials match.
    """
    row = db_conn().execute(
        "SELECT password_hash FROM users WHERE username = ?",
        (username,)
    ).fetchone()
    if not row:
        return False
    stored_hash = row["password_hash"]
    return checkpw(password.encode('utf-8'), stored_hash)

def remove_user_db(username: str) -> bool:
    """Deletes a user; returns True if a row was removed."""
    with db_conn() as conn:
        cur = conn.execute("DELETE FROM users WHERE username = ?", (username,))
        return cur.rowcount > 0
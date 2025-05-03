from sqlite3 import connect

DB_FILE = "/data/peers.db"


def db_conn():
    return connect(DB_FILE)


def init_db():
    with db_conn() as conn:
        conn.execute("""
            CREATE TABLE IF NOT EXISTS peers (
                public_key TEXT PRIMARY KEY,
                private_key TEXT,
                ipv4_address TEXT,
                ipv6_address TEXT,
                created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                expires_at TIMESTAMP
            )
        """)


def add_peer_db(pub, priv, ipv4, ipv6, expires):
    with db_conn() as conn:
        conn.execute("""
            INSERT INTO peers (public_key, private_key, ipv4_address, ipv6_address, expires_at)
            VALUES (?, ?, ?, ?, ?)
        """, (pub, priv, ipv4, ipv6, expires))


def remove_peer_db(pub):
    with db_conn() as conn:
        cur = conn.execute("DELETE FROM peers WHERE public_key = ?", (pub,))
        return cur.rowcount > 0


def get_all_peers():
    with db_conn() as conn:
        cur = conn.execute("SELECT public_key, ipv4_address, ipv6_address, created_at, expires_at FROM peers")
        return [dict(zip([column[0] for column in cur.description], row)) for row in cur.fetchall()]

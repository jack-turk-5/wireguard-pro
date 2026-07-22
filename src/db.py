import os
from sqlalchemy import create_engine, Column, String, LargeBinary, func
from sqlalchemy.orm import declarative_base, sessionmaker, Mapped
from sqlalchemy.exc import IntegrityError
from bcrypt import checkpw, hashpw, gensalt

DB_FILE = "/data/peers.db"
DATABASE_URL = f"sqlite:///{DB_FILE}"


def _ensure_dir():
    os.makedirs(os.path.dirname(DB_FILE), exist_ok=True)


_ensure_dir()

engine = create_engine(DATABASE_URL, connect_args={"check_same_thread": False})
SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)
Base = declarative_base()


class Peer(Base):
    __tablename__ = "peers"
    public_key: Mapped[str] = Column(String, primary_key=True)
    private_key: Mapped[str] = Column(String)
    ipv4_address: Mapped[str] = Column(String)
    ipv6_address: Mapped[str] = Column(String)
    created_at: Mapped[str] = Column(String, server_default=func.datetime("now"))
    expires_at: Mapped[str] = Column(String)


class User(Base):
    __tablename__ = "users"
    username: Mapped[str] = Column(String, primary_key=True)
    password_hash: Mapped[bytes] = Column(LargeBinary, nullable=False)
    created_at: Mapped[str] = Column(String, server_default=func.datetime("now"))


def init_db():
    Base.metadata.create_all(bind=engine)


def add_peer_db(pub: str, priv: str, ipv4: str, ipv6: str, expires):
    with SessionLocal() as session:
        peer = Peer(
            public_key=pub,
            private_key=priv,
            ipv4_address=ipv4,
            ipv6_address=ipv6,
            expires_at=expires,
        )
        session.add(peer)
        session.commit()


def remove_peer_db(pub_key: str) -> bool:
    with SessionLocal() as session:
        peer = session.query(Peer).filter(Peer.public_key == pub_key).first()
        if peer:
            session.delete(peer)
            session.commit()
            return True
        return False


def get_all_peers() -> list[Peer]:
    with SessionLocal() as session:
        peers = session.query(Peer).all()
        return peers


def add_user_db(username: str, password: str) -> bool:
    pwd_hash = hashpw(password.encode("utf-8"), gensalt())
    with SessionLocal() as session:
        user = User(username=username, password_hash=pwd_hash)
        session.add(user)
        try:
            session.commit()
            return True
        except IntegrityError:
            session.rollback()
            return False


def add_or_update_user_db(username: str, password: str) -> None:
    with SessionLocal() as session:
        user = session.query(User).filter(User.username == username).first()
        pwd_hash = hashpw(password.encode("utf-8"), gensalt())
        if user:
            user.password_hash = pwd_hash
        else:
            user = User(username=username, password_hash=pwd_hash)
            session.add(user)
        session.commit()


def verify_user_db(username: str, password: str) -> bool:
    with SessionLocal() as session:
        user = session.query(User).filter(User.username == username).first()
        if not user:
            return False

        return checkpw(password.encode("utf-8"), user.password_hash)


def remove_user_db(username: str) -> bool:
    with SessionLocal() as session:
        user = session.query(User).filter(User.username == username).first()
        if user:
            session.delete(user)
            session.commit()
            return True
        return False

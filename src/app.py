from datetime import datetime, timezone
import logging
from contextlib import asynccontextmanager
from os import environ, getloadavg
from time import gmtime, strftime
from typing import List

from fastapi import FastAPI, Depends, HTTPException, status
from fastapi.middleware.cors import CORSMiddleware
from fastapi.security import OAuth2PasswordBearer, OAuth2PasswordRequestForm
from itsdangerous import URLSafeTimedSerializer as Serializer, SignatureExpired, BadSignature
from pydantic import BaseModel, Field

from db import init_db, add_or_update_user_db, verify_user_db
from peers import create_peer, delete_peer, list_peers, peer_stats
from scheduler import scheduler
from utils import get_server_pubkey

# --- Configuration & Logging ---
logging.basicConfig(level=logging.DEBUG)
SECRET_KEY = environ.get('SECRET_KEY')
WG_SERVER_PUBKEY = get_server_pubkey()
WG_ENDPOINT = environ.get('WG_ENDPOINT')
WG_ALLOWED_IPS = environ.get('WG_ALLOWED_IPS', '0.0.0.0/0, ::/0')

if not SECRET_KEY or not WG_ENDPOINT:
    raise Exception('Missing SECRET_KEY and/or WG_ENDPOINT')

# --- Pydantic Models ---
class Token(BaseModel):
    access_token: str
    token_type: str

class DeletePeerRequest(BaseModel):
    public_key: str

class ServerInfo(BaseModel):
    uptime: str
    load: str

class ServerConfig(BaseModel):
    public_key: str
    endpoint: str
    allowed_ips: str

class Peer(BaseModel):
    public_key: str
    private_key: str
    ipv4_address: str
    ipv6_address: str
    expires_at: str
    created_at: str

class PeerCreate(BaseModel):
    days_valid: int = 7

class DeleteResponse(BaseModel):
    deleted: bool


# --- Token Management ---
ts = Serializer(SECRET_KEY, salt='auth-token')
oauth2_scheme = OAuth2PasswordBearer(tokenUrl="/api/login")

def generate_token(username: str) -> str:
    return ts.dumps({'user': username})

def verify_token(token: str = Depends(oauth2_scheme)) -> str:
    try:
        user = ts.loads(token, max_age=1800).get('user')
        if user is None:
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED,
                detail="Invalid token",
                headers={"WWW-Authenticate": "Bearer"},
            )
        return user
    except SignatureExpired:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Token has expired",
            headers={"WWW-Authenticate": "Bearer"},
        )
    except BadSignature:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid token signature",
            headers={"WWW-Authenticate": "Bearer"},
        )

# --- Lifespan Management (for startup/shutdown events) ---
@asynccontextmanager
async def lifespan(app: FastAPI):
    # Startup
    init_db()
    try:
        user = open('/run/secrets/admin-user').read().strip()
        pw = open('/run/secrets/admin-pass').read().strip()
        add_or_update_user_db(user, pw)
        logging.info(f"Seeded user `{user}`")
    except FileNotFoundError:
        logging.warning("Admin secrets not found. Skipping user seeding.")
    scheduler.start()
    yield
    # Shutdown
    scheduler.shutdown()

# --- FastAPI App ---
app = FastAPI(lifespan=lifespan)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["http://localhost:51819", "http://0.0.0.0:51819"], # Be more specific in production
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# --- API Endpoints ---
@app.post("/api/login", response_model=Token)
async def login(form_data: OAuth2PasswordRequestForm = Depends()):
    if not verify_user_db(form_data.username, form_data.password):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Incorrect username or password",
            headers={"WWW-Authenticate": "Bearer"},
        )
    access_token = generate_token(form_data.username)
    return {"access_token": access_token, "token_type": "bearer"}

@app.get("/api/config", response_model=ServerConfig)
async def get_config(current_user: str = Depends(verify_token)):
    return {
        "public_key": WG_SERVER_PUBKEY,
        "endpoint": WG_ENDPOINT,
        "allowed_ips": WG_ALLOWED_IPS,
    }

@app.post("/api/peers/new", response_model=Peer, status_code=status.HTTP_201_CREATED)
async def api_create_peer(req: PeerCreate, current_user: str = Depends(verify_token)):
    return create_peer(req.days_valid)

@app.post("/api/peers/delete", response_model=DeleteResponse)
async def api_delete_peer(req: DeletePeerRequest, current_user: str = Depends(verify_token)):
    return {"deleted": delete_peer(req.public_key)}

@app.get("/api/peers/list", response_model=List[Peer])
async def api_list_peers(current_user: str = Depends(verify_token)):
    return list_peers()

@app.get("/api/peers/stats")
async def api_peer_stats(current_user: str = Depends(verify_token)):
    return await peer_stats()

@app.get("/api/serverinfo", response_model=ServerInfo)
async def server_info(current_user: str = Depends(verify_token)):
    try:
        with open('/proc/uptime') as f:
            up = float(f.readline().split()[0])
        return {
            'uptime': strftime("%H:%M:%S", gmtime(up)),
            'load': "{:.2f} {:.2f} {:.2f}".format(*getloadavg())
        }
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

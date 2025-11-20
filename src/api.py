from typing import List
from fastapi import APIRouter, Depends, HTTPException, status, Request
from pydantic import BaseModel
from os import getloadavg
from time import gmtime, strftime

from peers import create_peer, delete_peer, list_peers, peer_stats
from auth import verify_token

router = APIRouter()

# --- Pydantic Models for API ---
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

# --- API Endpoints ---
@router.get("/config", response_model=ServerConfig)
async def get_config(request: Request, current_user: str = Depends(verify_token)):
    if not all([request.app.state.config.wg_public_key, request.app.state.config.wg_endpoint, request.app.state.config.wg_allowed_ips, request.app.state.config.wg_dns_server]):
        raise HTTPException(status_code=503, detail="Server configuration is not available due to a startup error.")
    return {
        "public_key": request.app.state.config.wg_public_key,
        "endpoint": request.app.state.config.wg_endpoint,
        "allowed_ips": request.app.state.config.wg_allowed_ips,
        "dns_server": request.app.state.config.wg_dns_server
    }

@router.post("/peers/new", response_model=Peer, status_code=status.HTTP_201_CREATED)
async def api_create_peer(req: PeerCreate, current_user: str = Depends(verify_token)):
    return await create_peer(req.days_valid)

@router.post("/peers/delete", response_model=DeleteResponse)
async def api_delete_peer(req: DeletePeerRequest, current_user: str = Depends(verify_token)):
    return {"deleted": await delete_peer(req.public_key)}

@router.get("/peers/list", response_model=List[Peer])
async def api_list_peers(current_user: str = Depends(verify_token)):
    return list_peers()

@router.get("/peers/stats")
async def api_peer_stats(current_user: str = Depends(verify_token)):
    return await peer_stats()

@router.get("/serverinfo", response_model=ServerInfo)
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

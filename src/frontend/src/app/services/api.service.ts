import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { Observable } from 'rxjs';

export interface Peer {
  private_key: string;
  public_key: string;
  ipv4_address: string;
  ipv6_address: string;
  expires_at?: string;
}

export interface Stat {
  public_key: string;
  last_handshake_time: number;
  rx_bytes: number;
  tx_bytes: number;
}

export interface ServerInfo {
  uptime: string;
  load: string;
}

@Injectable({ providedIn: 'root' })
export class ApiService {
  private jsonHeaders = { headers: new HttpHeaders({ 'Content-Type': 'application/json' }), withCredentials: true };

  constructor(private http: HttpClient) {}

  /** Create a new peer, returns the Peer object */  
  createPeer(daysValid: number = 7): Observable<Peer> {
    return this.http.post<Peer>('/api/peers/new', { days_valid: daysValid }, this.jsonHeaders);  
  }

  /** Delete a peer by public key, returns `{ deleted: boolean }` */  
  deletePeer(publicKey: string): Observable<{ deleted: boolean }> {
    return this.http.post<{ deleted: boolean }>('/api/peers/delete', { public_key: publicKey }, this.jsonHeaders);  
  }

  /** List all peers */  
  listPeers(): Observable<Peer[]> {
    return this.http.get<Peer[]>('/api/peers/list', { withCredentials: true });  
  }

  /** Get live peer stats */  
  getStats(): Observable<Stat[]> {
    return this.http.get<Stat[]>('/api/peers/stats', { withCredentials: true });  
  }

  /** Fetch server uptime & load average */  
  getServerInfo(): Observable<ServerInfo> {
    return this.http.get<ServerInfo>('/serverinfo', { withCredentials: true });  
  }
}
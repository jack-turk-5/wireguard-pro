import { Injectable } from '@angular/core';
import { Observable, of } from 'rxjs';
import { delay } from 'rxjs/operators';
import { Peer, ServerConfig, ServerHealthcheck, Stat } from './api.service';

// Demo mode: an in-memory stand-in for ApiService, swapped in via
// app.config.demo.ts (see angular.json's "demo" configuration) so the UI
// can be built/styled against realistic, varied data with zero backend --
// `npm run demo`. Not wired into the production bundle at all.

interface DemoPeer extends Peer {
  createdAt: number; // epoch seconds, used to derive fake stats
}

const NOW = Math.floor(Date.now() / 1000);

function fakeKey(seed: string): string {
  // Not a real WireGuard key -- just base64-alphabet, right length (44
  // chars incl. padding), so it renders/QR-encodes like the real thing.
  const raw = btoa(seed.padEnd(32, '0')).slice(0, 43);
  return raw + '=';
}

@Injectable({ providedIn: 'root' })
export class DemoApiService {
  private peers: DemoPeer[] = [
    {
      public_key: fakeKey('alice-laptop'),
      private_key: fakeKey('alice-laptop-priv'),
      // Left blank on purpose -- exercises the "falls back to the
      // wg-peer-<ip> filename" default display/download name.
      nickname: '',
      ipv4_address: '10.8.0.2',
      ipv6_address: 'fd86:ea04:1111::2',
      expires_at: new Date((NOW + 3 * 86400) * 1000).toISOString(),
      createdAt: NOW - 2 * 86400,
    },
    {
      public_key: fakeKey('bobs-phone'),
      private_key: fakeKey('bobs-phone-priv'),
      nickname: '',
      ipv4_address: '10.8.0.3',
      ipv6_address: 'fd86:ea04:1111::3',
      expires_at: new Date((NOW + 21 * 86400) * 1000).toISOString(),
      createdAt: NOW - 5 * 86400,
    },
    {
      public_key: fakeKey('homelab-nas'),
      private_key: fakeKey('homelab-nas-priv'),
      // Already renamed -- shows what a nicknamed peer looks like.
      nickname: 'Homelab NAS',
      ipv4_address: '10.8.0.4',
      ipv6_address: 'fd86:ea04:1111::4',
      // No expiry -- a permanent peer, exercises the "N/A" display branch.
      createdAt: NOW - 90 * 86400,
    },
    {
      public_key: fakeKey('new-tablet'),
      private_key: fakeKey('new-tablet-priv'),
      nickname: '',
      ipv4_address: '10.8.0.5',
      ipv6_address: 'fd86:ea04:1111::5',
      expires_at: new Date((NOW + 7 * 86400) * 1000).toISOString(),
      createdAt: NOW - 30, // just added, never connected -- no stats row
    },
  ];

  private nextIP = 6;

  createPeer(daysValid: number = 7): Observable<Peer> {
    const n = this.peers.length + 1;
    const peer: DemoPeer = {
      public_key: fakeKey(`new-peer-${n}-${Date.now()}`),
      private_key: fakeKey(`new-peer-${n}-priv-${Date.now()}`),
      nickname: '',
      ipv4_address: `10.8.0.${this.nextIP++}`,
      ipv6_address: `fd86:ea04:1111::${this.nextIP - 1}`,
      expires_at: new Date((NOW + daysValid * 86400) * 1000).toISOString(),
      createdAt: NOW,
    };
    this.peers.push(peer);
    return of(peer).pipe(delay(200));
  }

  deletePeer(publicKey: string): Observable<{ deleted: boolean }> {
    const before = this.peers.length;
    this.peers = this.peers.filter(p => p.public_key !== publicKey);
    return of({ deleted: this.peers.length < before }).pipe(delay(150));
  }

  renamePeer(publicKey: string, nickname: string): Observable<{ nickname: string }> {
    const peer = this.peers.find(p => p.public_key === publicKey);
    if (peer) {
      peer.nickname = nickname;
    }
    return of({ nickname }).pipe(delay(100));
  }

  listPeers(): Observable<Peer[]> {
    return of(this.peers.map(({ createdAt, ...p }) => p)).pipe(delay(150));
  }

  /** Live peer stats -- handshake/byte counters drift a little on every
   * call, so the demo's charts actually animate during dev work instead of
   * showing a flat line. */
  getStats(): Observable<Stat[]> {
    const now = Math.floor(Date.now() / 1000);
    const stats: Stat[] = this.peers
      // "new-tablet" has never connected -- no stats row, matching how a
      // real freshly-added peer looks before its first handshake.
      .filter(p => now - p.createdAt > 60)
      .map((p, i) => {
        const age = now - p.createdAt;
        // Rotates through good/warn/stale so all three td.* colors show up.
        const lastHandshakeAgo = [10, 180, 900][i % 3] + Math.floor(Math.random() * 10);
        const drift = Math.floor(age / 10) + Math.floor(Math.random() * 5);
        return {
          public_key: p.public_key,
          last_handshake_time: now - lastHandshakeAgo,
          rx_bytes: (2_500_000 + drift * 45_000) * (i + 1),
          tx_bytes: (900_000 + drift * 17_000) * (i + 1),
        };
      });
    return of(stats).pipe(delay(150));
  }

  getServerConfig(): Observable<ServerConfig> {
    return of({
      public_key: fakeKey('demo-server'),
      host: 'vpn.example.com',
      port: '51820',
      allowed_ips: '0.0.0.0/0, ::/0',
      dns_server: '1.1.1.1',
    }).pipe(delay(100));
  }

  getServerHealth(): Observable<ServerHealthcheck> {
    return of({
      uptime: '3 days, 4 hours',
      load: '0.15, 0.22, 0.19',
    }).pipe(delay(100));
  }
}

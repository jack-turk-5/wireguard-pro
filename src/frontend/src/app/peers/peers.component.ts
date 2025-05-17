import { Component, EventEmitter, OnInit, Output, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { QRCodeComponent } from 'angularx-qrcode';
import { ApiService } from '../services/api.service';

@Component({
  selector: 'app-peers',
  standalone: true,
  imports: [CommonModule, QRCodeComponent],
  templateUrl: './peers.component.html',
  styleUrls: []
})
export class PeersComponent implements OnInit {
  peers = signal<any[]>([]);
  @Output() qrClick = new EventEmitter<string>();

  constructor(private api: ApiService) {}

  ngOnInit() {
    this.loadPeers();
  }

  loadPeers() {
    this.api.listPeers().subscribe(list => this.peers.set(list));
  }

  trackByKey(_idx: number, peer: any) {
    return peer.public_key;
  }

  makeCfg(p: any): string {
    return [
      `[Interface]`,
      `PrivateKey = ${p.private_key}`,
      `Address = ${p.ipv4_address}/32`,
      ``,
      `[Peer]`,
      `PublicKey = YOUR_SERVER_PUBKEY`,
      `Endpoint = YOUR_ENDPOINT`,
      `AllowedIPs = 0.0.0.0/0, ::/0`,
      `PersistentKeepalive = 25`
    ].join('\n');
  }

  download(p: any) {
    const blob = new Blob([this.makeCfg(p)], { type: 'text/plain' });
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = `wg-peer-${p.ipv4_address}.conf`;
    a.click();
  }

  remove(key: string) {
    this.api.deletePeer(key).subscribe(() => this.loadPeers());
  }

  openQr(peer: any) {
    this.qrClick.emit(this.makeCfg(peer));
  }
  
}
import { Component, EventEmitter, OnInit, Output, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { QRCodeComponent } from 'angularx-qrcode';
import { ApiService, Peer, ServerConfig } from '../services/api.service';

@Component({
  selector: 'app-peers',
  standalone: true,
  imports: [CommonModule, QRCodeComponent],
  templateUrl: './peers.component.html',
  styles: `.qrcode {
  width: 80px; height: 80px; margin: auto;
}`
})
export class PeersComponent implements OnInit {
  peers = signal<any[]>([]);
  config: ServerConfig = {
    public_key:  '',
    endpoint:    '',
    allowed_ips: '',
    dns_server: ''
  };
  @Output() qrClick = new EventEmitter<string>();
  @Output() peerChange = new EventEmitter<void>();

  constructor(private api: ApiService) {}

  ngOnInit() {
    this.loadPeers();
    this.loadConfig();
  }

  loadPeers() {
    this.api.listPeers().subscribe(list => this.peers.set(list));
  }

  loadConfig() {
    this.api.getServerConfig().subscribe((config: ServerConfig) => this.config = config);
  }

  trackByKey(_idx: number, peer: any) {
    return peer.public_key;
  }

  makeCfg(p: Peer): string {
    return [
      `[Interface]`,
      `PrivateKey = ${p.private_key}`,
      `Address = ${p.ipv4_address}/32`,
      `DNS = ${this.config.dns_server}`,
      ``,
      `[Peer]`,
      `PublicKey = ${this.config.public_key}`,
      `Endpoint = ${this.config.endpoint}`,
      `AllowedIPs = ${this.config.allowed_ips}`,
      `PersistentKeepalive = 25`
    ].join('\n');
  }

  download(p: Peer) {
    const blob = new Blob([this.makeCfg(p)], { type: 'text/plain' });
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = `wg-peer-${p.ipv4_address}.conf`;
    a.click();
  }

  remove(key: string) {
    this.api.deletePeer(key).subscribe(res => {
      this.loadPeers();
      if (res.deleted) {
        this.peerChange.emit();
      }
    });
  }

  openQr(peer: Peer) {
    this.qrClick.emit(this.makeCfg(peer));
  }
  
}
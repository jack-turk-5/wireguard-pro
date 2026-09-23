import {
  Component,
  EventEmitter,
  OnInit,
  Output,
  signal,
  ChangeDetectionStrategy,
} from "@angular/core";
import { CommonModule } from "@angular/common";
import { QRCodeComponent } from "angularx-qrcode";
import { ApiService, Peer, ServerConfig } from "../services/api.service";

@Component({
  selector: "app-peers",
  standalone: true,
  imports: [CommonModule, QRCodeComponent],
  templateUrl: "./peers.component.html",
  changeDetection: ChangeDetectionStrategy.Eager,
})
export class PeersComponent implements OnInit {
  peers = signal<Peer[]>([]);
  config: ServerConfig = {
    public_key: "",
    host: "",
    port: "",
    allowed_ips: "",
    dns_server: "",
  };
  @Output() qrClick = new EventEmitter<string>();
  // Was "peerChange", while the parent template bound (peerDeleted) --
  // that binding was always a silent no-op (strictTemplates is off, so
  // Angular never caught the mismatch). Renamed to match what's actually
  // wired up, rather than leaving the dead binding in place.
  @Output() peerDeleted = new EventEmitter<void>();

  /** Public key currently shown in the view-key modal, if any. */
  viewedKey = signal<string | null>(null);
  /** Public key that was just copied, for brief button feedback. */
  copiedKey: string | null = null;
  private copiedTimer?: ReturnType<typeof setTimeout>;

  constructor(private api: ApiService) { }

  ngOnInit() {
    this.loadPeers();
    this.loadConfig();
  }

  loadPeers() {
    this.api.listPeers().subscribe((list) => this.peers.set(list));
  }

  loadConfig() {
    this.api.getServerConfig().subscribe((config: ServerConfig) => (this.config = config));
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
      `Endpoint = ${this.config.host}:${this.config.port}`,
      `AllowedIPs = ${this.config.allowed_ips}`,
      `PersistentKeepalive = 25`,
    ].join("\n");
  }

  /** What a peer's nickname defaults to until renamed -- also what the
   * downloaded filename falls back to when nickname is blank. */
  defaultName(p: Peer): string {
    return p.ipv4_address;
  }

  displayName(p: Peer): string {
    return p.nickname?.trim() || this.defaultName(p);
  }

  /** Persists a nickname edit; no-ops if unchanged. */
  rename(p: Peer, value: string) {
    const nickname = value.trim();
    if (nickname === (p.nickname ?? "")) {
      return;
    }
    this.api.renamePeer(p.public_key, nickname).subscribe(() => {
      p.nickname = nickname;
    });
  }

  download(p: Peer) {
    const blob = new Blob([this.makeCfg(p)], { type: "text/plain" });
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = `${this.displayName(p)}.conf`;
    a.click();
  }

  remove(key: string) {
    this.api.deletePeer(key).subscribe((res) => {
      this.loadPeers();
      if (res.deleted) {
        this.peerDeleted.emit();
      }
    });
  }

  openQr(peer: Peer) {
    this.qrClick.emit(this.makeCfg(peer));
  }

  viewKey(publicKey: string) {
    this.viewedKey.set(publicKey);
    document.body.classList.add("modal-open");
  }

  closeKeyModal() {
    this.viewedKey.set(null);
    document.body.classList.remove("modal-open");
  }

  async copyKey(publicKey: string) {
    try {
      await navigator.clipboard.writeText(publicKey);
    } catch {
      // Clipboard API can fail (permissions, non-secure context) -- the
      // View button is still there as a fallback to select/copy manually.
      return;
    }
    this.copiedKey = publicKey;
    clearTimeout(this.copiedTimer);
    this.copiedTimer = setTimeout(() => (this.copiedKey = null), 1500);
  }
}

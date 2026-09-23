import { Component, OnInit, signal, ViewChild, ChangeDetectionStrategy } from '@angular/core';
import { ApiService, ServerHealthcheck } from '../services/api.service';
import { AuthService } from '../services/auth.service';
import { ThemeService } from '../services/theme.service';
import { PeersComponent } from '../peers/peers.component';
import { StatsComponent } from '../stats/stats.component';
import { CommonModule } from '@angular/common';
import { Router } from '@angular/router';
import { QRCodeComponent } from 'angularx-qrcode';

@Component({
  selector: 'app-dashboard',
  standalone: true,
  imports: [CommonModule, PeersComponent, QRCodeComponent, StatsComponent],
  templateUrl: './dashboard.component.html',
  changeDetection: ChangeDetectionStrategy.Eager,
})
export class DashboardComponent implements OnInit {
  uptime  = 'Loading...';
  loadAvg = 'Loading...';
  zoomedQr = signal<string | null>(null);

  @ViewChild(PeersComponent) peersComp!: PeersComponent;
  @ViewChild(StatsComponent) statsComp!: StatsComponent;

  constructor(private api: ApiService, private auth: AuthService, private router: Router, public theme: ThemeService) {}

  ngOnInit() {
    this.fetchServerHealth();
    setInterval(() => this.fetchServerHealth(), 60_000);
  }

  logout() {
    this.auth.logout();
    this.router.navigate(['/login']);
  }

  /** Wrapper to create a peer, then reload the table in PeersComponent */
  addPeer() {
    this.api.createPeer(7).subscribe(() => {
      this.peersComp.loadPeers();
      this.refreshStats();
    });
  }

  /** Wrapper to refresh stats in StatsComponent */
  refreshStats() {
    this.statsComp.fetchAndUpdate();
  }

  private fetchServerHealth() {
    this.api.getServerHealth().subscribe((info: ServerHealthcheck) => {
      this.uptime  = info.uptime;
      this.loadAvg = info.load;
    });
  }

  onQrClick(cfg: string) {
    this.zoomedQr.set(cfg);
    document.body.classList.add('modal-open');
  }

  closeQr() {
    this.zoomedQr.set(null);
    document.body.classList.remove('modal-open');
  }

  onPeerChange() {
    this.refreshStats();
  }
}

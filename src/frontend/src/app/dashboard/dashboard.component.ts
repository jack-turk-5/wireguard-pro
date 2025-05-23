import { Component, EventEmitter, OnInit, Output, signal, ViewChild } from '@angular/core';
import { ApiService, ServerHealthcheck } from '../services/api.service';
import { AuthService } from '../services/auth.service';
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
  styleUrls: ['./dashboard.component.css']
})
export class DashboardComponent implements OnInit {
  uptime   = 'Loading...';
  loadAvg  = 'Loading...';
  isDarkMode = false;
  zoomedQr = signal<string|null>(null);
  @Output() peerAdded = new EventEmitter<void>();

  /** Expose ApiService publicly so template can use it */
  constructor(private api: ApiService, private auth: AuthService, private router: Router) {}

  /** Get a handle on the child components */
  @ViewChild(PeersComponent) peersComp!: PeersComponent;
  @ViewChild(StatsComponent) statsComp!: StatsComponent;

  ngOnInit() {
    // Initialize dark mode
    this.isDarkMode = window.matchMedia('(prefers-color-scheme: dark)').matches;
    document.body.classList.toggle('darkmode', this.isDarkMode);

    // Fetch server info immediately and every minute
    this.fetchServerHealth();
    setInterval(() => this.fetchServerHealth(), 60_000);
  }

  /** Toggle the page’s dark mode */
  toggleDarkMode() {
    this.isDarkMode = !this.isDarkMode;
    document.body.classList.toggle('darkmode', this.isDarkMode);
  }

  logout() {
    this.auth.logout();
    this.router.navigate(["/login"]);
  }

  /** Wrapper to create a peer, then reload the table in PeersComponent */
  addPeer() {
    this.api.createPeer(7).subscribe(() => {
      this.peersComp.loadPeers();
      this.peerAdded.emit();
    });
  }

  /** Wrapper to refresh stats in StatsComponent */
  refreshStats() {
    this.statsComp.fetchAndUpdate();
  }

  /** Internal call to fetch server uptime/load */
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
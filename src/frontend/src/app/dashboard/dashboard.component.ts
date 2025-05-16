// src/app/dashboard/dashboard.component.ts
import { Component, OnInit, ViewChild } from '@angular/core';
import { ApiService, ServerInfo } from '../services/api.service';
import { PeersComponent } from '../peers/peers.component';
import { StatsComponent } from '../stats/stats.component';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'app-dashboard',
  standalone: true,
  imports: [CommonModule, PeersComponent, StatsComponent],
  templateUrl: './dashboard.component.html',
  styleUrls: ['./dashboard.component.css']
})
export class DashboardComponent implements OnInit {
  uptime   = 'Loading...';
  loadAvg  = 'Loading...';
  isDarkMode = false;

  /** Expose ApiService publicly so template can use it */
  constructor(public api: ApiService) {}

  /** Get a handle on the child components */
  @ViewChild(PeersComponent) peersComp!: PeersComponent;
  @ViewChild(StatsComponent) statsComp!: StatsComponent;

  ngOnInit() {
    // Initialize dark mode
    this.isDarkMode = window.matchMedia('(prefers-color-scheme: dark)').matches;
    document.body.classList.toggle('darkmode', this.isDarkMode);

    // Fetch server info immediately and every minute
    this.fetchServerInfo();
    setInterval(() => this.fetchServerInfo(), 60_000);
  }

  /** Toggle the page’s dark mode */
  toggleDarkMode() {
    this.isDarkMode = !this.isDarkMode;
    document.body.classList.toggle('darkmode', this.isDarkMode);
  }

  /** Wrapper to create a peer, then reload the table in PeersComponent */
  addPeer() {
    this.api.createPeer(7).subscribe(() => {
      this.peersComp.loadPeers();
    });
  }

  /** Wrapper to refresh stats in StatsComponent */
  refreshStats() {
    this.statsComp.fetchAndUpdate();
  }

  /** Internal call to fetch server uptime/load */
  private fetchServerInfo() {
    this.api.getServerInfo().subscribe((info: ServerInfo) => {
      this.uptime  = info.uptime;
      this.loadAvg = info.load;
    });
  }
}
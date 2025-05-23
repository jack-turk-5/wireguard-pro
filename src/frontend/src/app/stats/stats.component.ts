import { Component, OnInit, ViewChildren, QueryList } from '@angular/core';
import { CommonModule } from '@angular/common';
import {
  ChartComponent,
  NgApexchartsModule,
  ApexAxisChartSeries,
  ApexChart,
  ApexXAxis,
  ApexDataLabels,
  ApexStroke,
  ApexTooltip
} from 'ng-apexcharts';
import { ApiService, Stat } from '../services/api.service';

@Component({
  selector: 'app-stats',
  standalone: true,
  imports: [CommonModule, NgApexchartsModule],
  templateUrl: './stats.component.html'
})
export class StatsComponent implements OnInit {
  @ViewChildren('chart') charts!: QueryList<ChartComponent>;

  stats: Stat[] = [];
  timeLabels: string[] = [];
  clientHistories: Record<string, { rx: number[]; tx: number[] }> = {};
  public chartConfig = {
    chart: { type: 'line', height: 300, animations: { enabled: true } } as ApexChart,
    stroke: { curve: 'smooth' } as ApexStroke,
    dataLabels: { enabled: false } as ApexDataLabels,
    tooltip: {
      theme: window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light',
      x: {
        show: true,
        formatter: (idx: number): string => this.timeLabels[idx] || ''
      },
      y: {
        title: {
          formatter: (seriesName: string): string => seriesName
        }
      }
    } as ApexTooltip
  };

  constructor(private api: ApiService) {}

  ngOnInit() {
    this.fetchAndUpdate();
    setInterval(() => this.fetchAndUpdate(), 10_000);
  }

  trackByKey(_: number, s: Stat): string {
    return s.public_key;
  }

  fetchAndUpdate(): void {
    this.api.getStats().subscribe(data => {
      const now = Math.floor(Date.now() / 1000);
      const updated = data.map(s => ({
        ...s,
        last_handshake_time: now - Number(s.last_handshake_time)
      }));

      this.stats.splice(0, this.stats.length, ...updated);
      const label = new Date().toLocaleTimeString();
      this.timeLabels = [...this.timeLabels, label].slice(-20);
      updated.forEach(s => {
        const mbRx = +(s.rx_bytes / 1e6).toFixed(2);
        const mbTx = +(s.tx_bytes / 1e6).toFixed(2);
        if (!this.clientHistories[s.public_key]) {
          this.clientHistories[s.public_key] = { rx: [], tx: [] };
        }
        const hist = this.clientHistories[s.public_key];
        hist.rx = [...hist.rx, mbRx].slice(-20);
        hist.tx = [...hist.tx, mbTx].slice(-20);
      });
    });
  }
}
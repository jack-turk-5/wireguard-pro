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
    series: [
      { name: 'Received (MB)', data: [] },
      { name: 'Sent (MB)', data: [] }
    ] as ApexAxisChartSeries,
    chart: {
      type: 'line',
      height: 300,
      animations: { enabled: true }
    } as ApexChart,
    stroke: { curve: 'smooth' } as ApexStroke,
    dataLabels: { enabled: false } as ApexDataLabels,
    xaxis: { categories: [] } as ApexXAxis,
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
    this.api.getStats().subscribe(raw => {
      const now = Math.floor(Date.now() / 1000);
      const updated = raw.map(s => ({
        ...s,
        last_handshake_time: now - Number(s.last_handshake_time)
      }));

      this.stats.splice(0, this.stats.length, ...updated);

      const label = new Date().toLocaleTimeString();
      this.timeLabels = [...this.timeLabels, label].slice(-20);

      updated.forEach(s => {
        const mbRx = +(s.rx_bytes / 1e6).toFixed(2);
        const mbTx = +(s.tx_bytes / 1e6).toFixed(2);
        const key  = s.public_key;
        if (!this.clientHistories[key]) {
          this.clientHistories[key] = { rx: [], tx: [] };
        }
        const hist = this.clientHistories[key];
        hist.rx = [...hist.rx, mbTx].slice(-20);
        hist.tx = [...hist.tx, mbRx].slice(-20);
      });

      this.charts.forEach((chartComp, idx) => {
        const peer = this.stats[idx];
        if (!peer) return;

        const hist = this.clientHistories[peer.public_key];
        chartComp.updateSeries(
          [
            { name: 'Received (MB)', data: hist.rx },
            { name: 'Sent (MB)', data: hist.tx }
          ],
          false
        );
        chartComp.updateOptions(
          { xaxis: { categories: this.timeLabels } },
          false,
          false
        );
      });
    });
  }
}
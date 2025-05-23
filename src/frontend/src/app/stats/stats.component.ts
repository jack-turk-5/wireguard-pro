// stats.component.ts
import { Component, OnInit, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import {
  ChartComponent,
  NgApexchartsModule,
  ApexAxisChartSeries,
  ApexChart,
  ApexXAxis,
  ApexDataLabels,
  ApexStroke,
  ApexTitleSubtitle,
  ApexTooltip
} from 'ng-apexcharts';
import { ApiService, Stat } from '../services/api.service';

export type ChartOptions = {
  series: ApexAxisChartSeries;
  chart: ApexChart;
  xaxis: ApexXAxis;
  stroke: ApexStroke;
  dataLabels: ApexDataLabels;
  title: ApexTitleSubtitle;
  tooltip: ApexTooltip;
};

@Component({
  selector: 'app-stats',
  standalone: true,
  imports: [CommonModule, NgApexchartsModule],
  templateUrl: './stats.component.html'
})
export class StatsComponent implements OnInit {
  @ViewChild('chart') chart!: ChartComponent;

  stats: Stat[] = [];
  private history: Record<string, number[]> = {};   // public_key → [rxMB, …]
  private timeLabels: string[] = [];

  public chartOptions: ChartOptions = {
    series: [],
    chart: { type: 'line', height: 350 },
    stroke: { curve: 'smooth' },
    dataLabels: { enabled: false },
    xaxis: { categories: [] },
    title: { text: 'Per‐Peer RX (MB)', align: 'left' },
    tooltip: {
      theme: window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light',
      x: { show: false },     // hide the “index/label” header
      y: {                   // each row’s title will be the seriesName
        title: {
          formatter: (seriesName: string): string => seriesName
        }
      }
    }
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
        const mb = s.rx_bytes / 1e6;
        if (!this.history[s.public_key]) this.history[s.public_key] = [];
        this.history[s.public_key] = [...this.history[s.public_key], +mb.toFixed(2)].slice(-20);
      });

      const series = Object.entries(this.history).map(([key, data]) => ({
        name: key,
        data
      }));

      this.chart.updateSeries(series, false);
      this.chart.updateOptions(
        { xaxis: { categories: this.timeLabels } },
        false,
        false
      );
    });
  }
}
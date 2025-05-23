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
  private rxHistory: number[] = [];
  private txHistory: number[] = [];
  private timeLabels: string[] = [];

  public chartOptions: ChartOptions = {
    series: [
      { name: 'RX (MB)', data: [] },
      { name: 'TX (MB)', data: [] }
    ],
    chart: {
      type: 'line',
      height: 350,
      animations: { enabled: true }
    },
    stroke: { curve: 'smooth' },
    dataLabels: { enabled: false },
    xaxis: { categories: [] },
    title: { text: '', align: 'left' },
    tooltip: {
      theme: window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light',
      x: {
        show: true,
        formatter: (val: number, _opts?: any): string => {
          // val is the index into our timeLabels array
          return this.timeLabels[val] || '';
        }
      },
      y: {
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
      const totalRx = +(
        updated.reduce((sum, s) => sum + s.rx_bytes, 0) / 1e6
      ).toFixed(2);
      const totalTx = +(
        updated.reduce((sum, s) => sum + s.tx_bytes, 0) / 1e6
      ).toFixed(2);
      const label = new Date().toLocaleTimeString();

      this.timeLabels = [...this.timeLabels, label].slice(-20);
      this.rxHistory   = [...this.rxHistory, totalRx].slice(-20);
      this.txHistory   = [...this.txHistory, totalTx].slice(-20);

      this.chart.updateSeries(
        [
          { name: 'RX (MB)', data: this.rxHistory },
          { name: 'TX (MB)', data: this.txHistory }
        ],
        false
      );

      this.chart.updateOptions(
        {
          xaxis: { categories: this.timeLabels },
          title: { text: updated[0]?.public_key ?? '' }
        },
        false,
        false
      );
    });
  }
}
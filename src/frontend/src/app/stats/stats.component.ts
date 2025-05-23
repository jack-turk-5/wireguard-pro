// stats.component.ts
import { Component, OnInit, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import {
  NgApexchartsModule,
  ChartComponent,
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
    title: { text: 'VPN Traffic (MB)', align: 'left' },
    tooltip: {
      theme: window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light',  
      x: {
        show: true,
        formatter: (val: number, opts?: any): string => {
          return String(val);
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

      const totalRx = updated.reduce((sum, s) => sum + s.rx_bytes, 0) / 1e6;
      const totalTx = updated.reduce((sum, s) => sum + s.tx_bytes, 0) / 1e6;
      const label   = new Date().toLocaleTimeString();

      const prevRx = this.chartOptions.series[0].data as number[];
      const prevTx = this.chartOptions.series[1].data as number[];
      const prevCat= this.chartOptions.xaxis.categories as string[];

      const rxData = [...prevRx, +totalRx.toFixed(2)].slice(-20);
      const txData = [...prevTx, +totalTx.toFixed(2)].slice(-20);
      const cats   = [...prevCat, label].slice(-20);

      (this.chartOptions.series[0].data as number[]) = rxData;
      (this.chartOptions.series[1].data as number[]) = txData;
      (this.chartOptions.xaxis.categories as string[]) = cats;
    });
  }
}
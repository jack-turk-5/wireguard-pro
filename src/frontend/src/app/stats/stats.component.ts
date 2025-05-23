// stats.component.ts
import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import {
  NgApexchartsModule,
  ApexAxisChartSeries,
  ApexChart,
  ApexXAxis,
  ApexDataLabels,
  ApexStroke,
  ApexTitleSubtitle
} from 'ng-apexcharts';
import { ApiService, Stat } from '../services/api.service';

export type ChartOptions = {
  series: ApexAxisChartSeries;
  chart: ApexChart;
  xaxis: ApexXAxis;
  stroke: ApexStroke;
  dataLabels: ApexDataLabels;
  title: ApexTitleSubtitle;
};

@Component({
  selector: 'app-stats',
  standalone: true,
  imports: [CommonModule, NgApexchartsModule],
  templateUrl: './stats.component.html',
  styleUrls: []
})
export class StatsComponent implements OnInit {
  stats: Stat[] = [];

  public chartOptions: Partial<ChartOptions> = {
    series: [
      { name: 'RX (MB)', data: [] },
      { name: 'TX (MB)', data: [] }
    ],
    chart: { type: 'line', height: 350 },
    stroke: { curve: 'smooth' },
    dataLabels: { enabled: false },
    xaxis: { categories: [] },
    title: { text: 'VPN Traffic (MB)', align: 'left' }
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

      // 1) Update stats in place
      this.stats.splice(0, this.stats.length, ...updated);

      // 2) Compute totals & new label
      const totalRx = updated.reduce((sum, s) => sum + s.rx_bytes, 0) / 1e6;
      const totalTx = updated.reduce((sum, s) => sum + s.tx_bytes, 0) / 1e6;
      const label   = new Date().toLocaleTimeString();

      // 3) Build new series & categories (keep last 20 points)
      const prevRx  = this.chartOptions.series![0].data as number[];
      const prevTx  = this.chartOptions.series![1].data as number[];
      const prevCat = this.chartOptions.xaxis!.categories as string[];

      const rxData = [...prevRx, +totalRx.toFixed(2)].slice(-20);
      const txData = [...prevTx, +totalTx.toFixed(2)].slice(-20);
      const cats   = [...prevCat, label].slice(-20);

      // 4) Mutate chartOptions in place
      (this.chartOptions.series![0].data as number[])    = rxData;
      (this.chartOptions.series![1].data as number[])    = txData;
      (this.chartOptions.xaxis!.categories as string[]) = cats;
    });
  }
}
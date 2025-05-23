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
  templateUrl: './stats.component.html'
})
export class StatsComponent implements OnInit {
  stats: Stat[] = [];

  public chartOptions: ChartOptions = {
    series: [
      { name: 'RX (MB)', data: [] },
      { name: 'TX (MB)', data: [] }
    ],
    chart: { type: 'line', height: 350, animations: { enabled: true } },
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

  trackByKey(index: number, s: Stat): string {
    return s.public_key;
  }

  private fetchAndUpdate(): void {
    this.api.getStats().subscribe(data => {
      const now = Math.floor(Date.now() / 1000);
      const updated = data.map(s => ({
        ...s,
        last_handshake_time: now - Number(s.last_handshake_time)
      }));

      // update table in place
      this.stats.splice(0, this.stats.length, ...updated);

      // compute chart values
      const totalRx = updated.reduce((acc, s) => acc + s.rx_bytes, 0) / 1e6;
      const totalTx = updated.reduce((acc, s) => acc + s.tx_bytes, 0) / 1e6;
      const label   = new Date().toLocaleTimeString();

      const rxPrev = this.chartOptions.series[0].data as number[];
      const txPrev = this.chartOptions.series[1].data as number[];
      const catPrev= this.chartOptions.xaxis.categories as string[];

      const rx     = [...rxPrev, +totalRx.toFixed(2)].slice(-20);
      const tx     = [...txPrev, +totalTx.toFixed(2)].slice(-20);
      const cats   = [...catPrev, label].slice(-20);

      // mutate chart in place
      (this.chartOptions.series[0].data as number[])     = rx;
      (this.chartOptions.series[1].data as number[])     = tx;
      (this.chartOptions.xaxis.categories as string[])   = cats;
    });
  }
}
import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import {
  ApexAxisChartSeries,
  ApexChart,
  ApexXAxis,
  ApexDataLabels,
  ApexStroke,
  ApexTitleSubtitle,
  NgApexchartsModule
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
  public chartOptions: Partial<ChartOptions>;

  constructor(private api: ApiService) {
    // Initialize chart options with empty series
    this.chartOptions = {
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
  }

  ngOnInit() {
    this.fetchAndUpdate();                                  
    setInterval(() => this.fetchAndUpdate(), 10_000);       
  }

  fetchAndUpdate(): void {
    this.api.getStats().subscribe((data: Stat[]) => {
      this.stats = data;                                    

      const now = new Date().toLocaleTimeString();
      const totalRx = data.reduce((sum, s) => sum + s.rx_bytes, 0) / 1e6;
      const totalTx = data.reduce((sum, s) => sum + s.tx_bytes, 0) / 1e6;

      // Append new points and keep only the last 20
      const rxSeries = [...(this.chartOptions.series![0].data as number[]), +totalRx.toFixed(2)].slice(-20);
      const txSeries = [...(this.chartOptions.series![1].data as number[]), +totalTx.toFixed(2)].slice(-20);
      const categories = [...(this.chartOptions.xaxis!.categories as string[]), now].slice(-20);

      this.chartOptions = {
        ...this.chartOptions,
        series: [
          { name: 'RX (MB)', data: rxSeries },
          { name: 'TX (MB)', data: txSeries }
        ],
        xaxis: { categories }
      };                                                    
    });
  }
}
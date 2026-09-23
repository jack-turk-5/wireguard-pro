import { Component, ChangeDetectionStrategy } from '@angular/core';
import { RouterOutlet } from '@angular/router';
import { ThemeService } from './services/theme.service';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [RouterOutlet],
  template: `
    <header class="bg-primary-hover px-4 py-6 text-center text-white shadow-sm">
      <h1 class="text-xl sm:text-2xl">WireGuard Pro 🛡️ Rootless VPN Dashboard</h1>
    </header>

    <main class="mx-auto max-w-5xl p-4">
      <router-outlet></router-outlet>
    </main>
  `,
  changeDetection: ChangeDetectionStrategy.Eager,
})
export class AppComponent {
  // Injected purely to instantiate ThemeService (and apply the initial
  // light/dark class to <html>) as early as possible -- see its own doc
  // comment for why this replaces what used to be separate, inconsistent
  // dark-mode logic here and in DashboardComponent.
  constructor(private theme: ThemeService) {}
}

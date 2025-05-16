// src/app/app.component.ts
import { Component } from '@angular/core';
import { RouterModule } from '@angular/router';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [RouterModule],
  template: `
    <!-- Optional top‐bar or nav goes here -->
    <header class="app-header">
      <h1>WireGuard Pro Dashboard</h1>
    </header>

    <!-- Routed views render here -->
    <main>
      <router-outlet></router-outlet>
    </main>
  `,
  /* styles: [`
    .app-header {
      background: #0d6efd;
      color: white;
      padding: 1rem;
      text-align: center;
    }
    main {
      padding: 1rem;
    }
  `] */
})
export class AppComponent {}
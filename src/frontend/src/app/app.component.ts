// src/app/app.component.ts
import { Component } from '@angular/core';
import { RouterOutlet } from '@angular/router';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [RouterOutlet],
  template: `
    <header class="banner app-header">
      <h1>WireGuard Pro Dashboard</h1>
    </header>

    <main class="container">
      <router-outlet></router-outlet>
    </main>
  `,
  styles: [`
    /* match dashboard banner */
    .app-header {
      background: #0d6efd;
      color: white;
      padding: 1rem;
      text-align: center;
    }

    /* ensure main content has the right padding */
    main {
      padding: 1rem;
    }

    /* reuse your existing global .toggle-darkmode style */
    .toggle-darkmode {
      margin-top: 0.5rem;
      background: transparent;
      border: 1px solid white;
      color: white;
      padding: 0.5rem 1rem;
      border-radius: 4px;
      cursor: pointer;
      font-size: 0.9rem;
      transition: background-color 0.2s;
    }
    .toggle-darkmode:hover {
      background-color: rgba(255, 255, 255, 0.2);
    }
  `]
})
export class AppComponent {
  toggleDarkMode() {
    document.body.classList.toggle('darkmode');
  }
}

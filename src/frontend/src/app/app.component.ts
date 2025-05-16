import { Component, OnInit } from '@angular/core';
import { RouterOutlet } from '@angular/router';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [RouterOutlet],
  template: `
    <header class="banner app-header">
      <h1>WireGuard Pro 🛡️ Rootless VPN Dashboard</h1>
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

    button {
      background-color: #198754; color: white;
      border: none; padding: 8px 12px;
      cursor: pointer; border-radius: 4px;
      transition: background-color 0.2s;
    }

    button:hover {
      background-color: #145c32;
    }

    /* ensure main content has the right padding */
    main {
      padding: 1rem;
    }

    /* (you can still call this manually if you add a button later) */
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
export class AppComponent implements OnInit {
  ngOnInit() {
    // Apply system default theme on startup
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
    if (prefersDark) {
      document.body.classList.add('darkmode');
    } else {
      document.body.classList.remove('darkmode');
    }

    // Optional: react to changes while the app is open
    window.matchMedia('(prefers-color-scheme: dark)')
      .addEventListener('change', e => {
        document.body.classList.toggle('darkmode', e.matches);
      });
  }

  toggleDarkMode() {
    document.body.classList.toggle('darkmode');
  }
}
import { Injectable, signal } from '@angular/core';

const STORAGE_KEY = 'wireguard-pro-theme';

/** Single source of truth for dark mode -- previously duplicated
 * (inconsistently) between AppComponent and DashboardComponent, neither of
 * which persisted an explicit user choice across reloads. Toggling adds/
 * removes `dark` on <html>, matching Tailwind's dark: variant selector
 * (see the @custom-variant in styles.css). */
@Injectable({ providedIn: 'root' })
export class ThemeService {
  readonly isDark = signal(this.resolveInitial());

  constructor() {
    this.apply(this.isDark());
    // Only follow the OS preference live if the user never made an
    // explicit choice -- an explicit toggle should stick.
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', e => {
      if (localStorage.getItem(STORAGE_KEY) === null) {
        this.isDark.set(e.matches);
        this.apply(e.matches);
      }
    });
  }

  toggle(): void {
    const next = !this.isDark();
    this.isDark.set(next);
    localStorage.setItem(STORAGE_KEY, next ? 'dark' : 'light');
    this.apply(next);
  }

  private resolveInitial(): boolean {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored !== null) {
      return stored === 'dark';
    }
    return window.matchMedia('(prefers-color-scheme: dark)').matches;
  }

  private apply(dark: boolean): void {
    document.documentElement.classList.toggle('dark', dark);
  }
}

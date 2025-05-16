import { InjectionToken } from '@angular/core';
export const LOCAL_STORAGE = new InjectionToken<Storage | null>(
    'local-storage',
    {
        providedIn: 'root',
        factory: () => {
          // Check if window/localStorage is available
          if (typeof window !== 'undefined' && window.localStorage) {
            return window.localStorage;
          }
          // Return null if not in a browser context
          return null;
        }
      }
);
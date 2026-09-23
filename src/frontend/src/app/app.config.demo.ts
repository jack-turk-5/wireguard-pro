import { ApplicationConfig, provideZoneChangeDetection } from '@angular/core';
import { provideRouter } from '@angular/router';
import { routes } from './app.routes';
import { provideClientHydration, withEventReplay, withNoIncrementalHydration } from '@angular/platform-browser';
import { provideHttpClient, withFetch, withInterceptors } from '@angular/common/http';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { JwtInterceptor } from './auth/jwt.interceptor';
import { ApiService } from './services/api.service';
import { DemoApiService } from './services/api.service.demo';
import { AuthService } from './services/auth.service';
import { DemoAuthService } from './services/auth.service.demo';

// Swapped in for app.config.ts via angular.json's "demo" build
// configuration (fileReplacements) -- `npm run demo`. Only the two data
// services differ; everything else (routing, hydration, the JWT
// interceptor) stays identical so the app behaves the same shape it does
// in production, just against in-memory data instead of the real API.
export const appConfig: ApplicationConfig = {
  providers: [
    provideZoneChangeDetection({ eventCoalescing: true }),
    provideRouter(routes), provideClientHydration(withEventReplay(), withNoIncrementalHydration()),
    provideHttpClient(
      withInterceptors([JwtInterceptor]),
      withFetch()
    ),
    { provide: ApiService, useClass: DemoApiService },
    { provide: AuthService, useClass: DemoAuthService },
  ]
}

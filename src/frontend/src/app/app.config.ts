import { ApplicationConfig, PLATFORM_ID, provideZoneChangeDetection } from '@angular/core';
import { provideRouter } from '@angular/router';
import { routes } from './app.routes';
import { provideClientHydration, withEventReplay } from '@angular/platform-browser';
import { provideHttpClient, withFetch, withInterceptors } from '@angular/common/http';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { JwtInterceptor } from './auth/jwt.interceptor';
import { LOCAL_STORAGE } from './storage/storage';
import { isPlatformServer } from '@angular/common';

export const appConfig: ApplicationConfig = {
  providers: [
    provideZoneChangeDetection({ eventCoalescing: true }), 
    provideRouter(routes), provideClientHydration(withEventReplay()), 
    provideHttpClient(
      withInterceptors([JwtInterceptor]), 
      withFetch()
    ), 
    provideAnimationsAsync(),
    {
      provide: LOCAL_STORAGE,
      useFactory: (platformId: object) => {
   	 if (isPlatformServer(platformId)) {
   	   return {}; // Return an empty object on the server
   	 }
   	 return localStorage; // Use the browser's localStorage
      },
      deps: [PLATFORM_ID],
    }
  ]
}

// src/main.ts
import { bootstrapApplication } from '@angular/platform-browser';
import { provideRouter }       from '@angular/router';
import { provideHttpClient, withInterceptors } from '@angular/common/http';

import { AppComponent } from './app/app.component';
import { JwtInterceptor } from './app/auth/jwt.interceptor';
import { routes } from './app/app.routes';

bootstrapApplication(AppComponent, {
  providers: [
    provideRouter(routes),
    provideHttpClient(
      withInterceptors([JwtInterceptor])
    )
  ]
})
.catch(err => console.error(err));
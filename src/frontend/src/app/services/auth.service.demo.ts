import { Injectable } from '@angular/core';
import { Observable, of, throwError } from 'rxjs';
import { delay } from 'rxjs/operators';
import { AuthResponse } from './auth.service';

// Demo mode stand-in for AuthService -- see api.service.demo.ts. Accepts
// any credentials except password "wrong", which simulates a 401 so the
// login page's error state is also visible in demo mode.
@Injectable({ providedIn: 'root' })
export class DemoAuthService {
  private readonly TOKEN_KEY = 'demo-jwt-token';

  login(creds: { username: string; password: string }): Observable<AuthResponse> {
    if (creds.password === 'wrong') {
      return throwError(() => ({ status: 401 })).pipe(delay(200));
    }
    const res: AuthResponse = { access_token: 'demo-token', token_type: 'bearer' };
    localStorage.setItem(this.TOKEN_KEY, res.access_token);
    return of(res).pipe(delay(200));
  }

  getToken(): string | null {
    return localStorage.getItem(this.TOKEN_KEY);
  }

  logout(): void {
    localStorage.removeItem(this.TOKEN_KEY);
  }
}

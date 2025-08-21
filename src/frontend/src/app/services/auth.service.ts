import { Injectable } from '@angular/core';
import { tap } from 'rxjs';
import { HttpClient, HttpHeaders, HttpParams } from '@angular/common/http';

export interface AuthResponse {
  access_token: string;
  token_type: string;
}

@Injectable({ providedIn: 'root' })
export class AuthService {
  private readonly TOKEN_KEY = 'jwt-token';

  constructor(private httpClient: HttpClient) {}

  login(creds: { username: string; password: string }) {
    const body = new HttpParams()
      .set('username', creds.username)
      .set('password', creds.password);

    return this.httpClient.post<AuthResponse>('/api/login', body).pipe(
      tap(res => {
        localStorage.setItem(this.TOKEN_KEY, res.access_token);
      })
    );
  }

  getToken(): string | null {
    return localStorage.getItem(this.TOKEN_KEY);
  }

  logout(): void {
    localStorage.removeItem(this.TOKEN_KEY);
  }
}
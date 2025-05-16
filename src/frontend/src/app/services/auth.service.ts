import { inject, Injectable } from '@angular/core';
import { BehaviorSubject, Observable, tap } from 'rxjs';
import { LOCAL_STORAGE } from '../storage/storage';
import { HttpClient } from '@angular/common/http';

export interface AuthResponse {
  token: string;
}

@Injectable({
  providedIn: 'root'
})
export class AuthService {
  private readonly TOKEN_KEY = 'jwt-token';
  private readonly loggedInSubject = new BehaviorSubject<boolean>(this.hasToken());
  private readonly storage = inject(LOCAL_STORAGE);

  constructor(private readonly httpClient: HttpClient) {}

  login(credentials: { username: string; password: string }): Observable<{ token: string }> {
    return this.httpClient.post<AuthResponse>('/api/login', credentials).pipe(
      tap(response => {
        this.storeToken(response.token);
        this.loggedInSubject.next(true);
      })
    );
  }

  logout(): void {
    this.storage?.removeItem(this.TOKEN_KEY);
    this.loggedInSubject.next(false);
  }

  isLoggedIn(): Observable<boolean> {
    return this.loggedInSubject.asObservable();
  }

  getToken(): string | null {
    return this.storage?.getItem(this.TOKEN_KEY) ?? null;
  }

  private storeToken(token: string): void {
    this.storage?.setItem(this.TOKEN_KEY, token);
  }

  private hasToken(): boolean {
    return this.storage?.getItem(this.TOKEN_KEY) != null;
  }
}
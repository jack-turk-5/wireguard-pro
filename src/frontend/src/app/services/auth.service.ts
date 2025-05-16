import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { BehaviorSubject, tap } from 'rxjs';

@Injectable({ providedIn: 'root' })
export class AuthService {
  private token$ = new BehaviorSubject<string|null>(null);

  constructor(private http: HttpClient) {}

  login(user: string, pass: string) {
    return this.http.post<{ token: string }>(
      '/api/login',
      { username: user, password: pass }
    ).pipe(
      tap(res => this.token$.next(res.token))
    );
  }

  get token(): string|null {
    return this.token$.value;
  }

  logout() {
    this.token$.next(null);
  }
}
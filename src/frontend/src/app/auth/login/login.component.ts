import { Component } from '@angular/core';
import { Router } from '@angular/router';
import { CommonModule } from '@angular/common';
import {
  ReactiveFormsModule,
  FormGroup,
  FormControl,
  Validators
} from '@angular/forms';
import { AuthService } from '../../services/auth.service';

@Component({
  selector: 'app-login',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule],
  template: `
    <form [formGroup]="loginForm" (ngSubmit)="submit()">
      <div class="error" *ngIf="errorMessage">
        {{ errorMessage }}
      </div>
      <label for="user">Username</label>
      <input
        id="user"
        type="text"
        formControlName="user"
        placeholder="Username"
        (focus)="clearError()"
      />
      <div class="error-inline" *ngIf="loginForm.controls.user.invalid && loginForm.controls.user.touched">
        Username is required.
      </div>

      <label for="pass">Password</label>
      <input
        id="pass"
        type="password"
        formControlName="pass"
        placeholder="Password"
        (focus)="clearError()"
      />
      <div class="error-inline" *ngIf="loginForm.controls.pass.invalid && loginForm.controls.pass.touched">
        Password is required.
      </div>

      <button type="submit" [disabled]="loginForm.invalid">
        Login
      </button>
    </form>
  `,
  styles: [`
    form {
      display: flex;
      flex-direction: column;
      gap: 10px;
      max-width: 320px;
      margin: 50px auto;
    }
    label {
      font-weight: bold;
    }
    input {
      padding: 8px;
      font-size: 1rem;
      border: 1px solid #ccc;
      border-radius: 4px;
    }
    .error {
      color: #d9534f;
      font-size: 0.875rem;
      margin-bottom: 10px;
      padding: 10px;
      border-radius: 4px;
      background-color: #f2dede;
      border: 1px solid #ebccd1;
      text-align: center;
    }
    .error-inline {
      color: #d9534f;
      font-size: 0.875rem;
    }
    button {
      margin-top: 1rem;
      background-color: #198754; color: white;
      border: none; padding: 8px 12px;
      cursor: pointer; border-radius: 4px;
      transition: background-color 0.2s;
    }

    button:hover {
      background-color: #145c32;
    }
  `]
})
export class LoginComponent {
  errorMessage: string | null = null;

  // A FormGroup with non-nullable FormControls<string>
  loginForm = new FormGroup({
    user: new FormControl<string>('', {
      nonNullable: true,
      validators: Validators.required
    }),
    pass: new FormControl<string>('', {
      nonNullable: true,
      validators: Validators.required
    })
  });

  constructor(
    private auth: AuthService,
    private router: Router
  ) {}

  submit() {
    if (this.loginForm.invalid) {
      this.loginForm.markAllAsTouched();
      return;
    }
    this.clearError();

    // Use getRawValue() so `user` and `pass` are typed as string, not string|undefined
    const { user, pass } = this.loginForm.getRawValue();

    this.auth.login({ username: user, password: pass }).subscribe({
      next: () => {
        this.router.navigate(['/dashboard']);
      },
      error: (err) => {
        if (err.status === 401) {
          this.errorMessage = 'Incorrect username or password.';
        } else if (err.status >= 500) {
          this.errorMessage = 'A server error occurred. Please try again later.';
        } else {
          this.errorMessage = 'An unexpected error occurred. Please try again.';
        }
      }
    });
  }

  clearError() {
    this.errorMessage = null;
  }
}
// src/app/auth/login.component.ts
import { Component } from '@angular/core';
import { Router } from '@angular/router';
import { CommonModule } from '@angular/common';
import {
  ReactiveFormsModule,
  FormGroup,
  FormControl,
  Validators
} from '@angular/forms';
import { AuthService } from '../auth.service';

@Component({
  selector: 'app-login',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule],
  template: `
    <form [formGroup]="loginForm" (ngSubmit)="submit()">
      <label for="user">Username</label>
      <input
        id="user"
        type="text"
        formControlName="user"
        placeholder="Username"
      />
      <div class="error" *ngIf="loginForm.controls.user.invalid && loginForm.controls.user.touched">
        Username is required.
      </div>

      <label for="pass">Password</label>
      <input
        id="pass"
        type="password"
        formControlName="pass"
        placeholder="Password"
      />
      <div class="error" *ngIf="loginForm.controls.pass.invalid && loginForm.controls.pass.touched">
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
    }
    button {
      padding: 10px;
      font-size: 1rem;
      background-color: #0d6efd;
      color: white;
      border: none;
      border-radius: 4px;
      cursor: pointer;
      transition: opacity 0.2s;
    }
    button[disabled] {
      opacity: 0.6;
      cursor: not-allowed;
    }
  `]
})
export class LoginComponent {
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

    // Use getRawValue() so `user` and `pass` are typed as string, not string|undefined
    const { user, pass } = this.loginForm.getRawValue();

    this.auth.login(user, pass).subscribe(() => {
      this.router.navigate(['/dashboard']);
    });
  }
}
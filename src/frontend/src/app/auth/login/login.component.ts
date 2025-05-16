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
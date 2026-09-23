import { Component, ChangeDetectionStrategy } from '@angular/core';
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
    <form class="card mx-auto mt-16 flex max-w-sm flex-col gap-4" [formGroup]="loginForm" (ngSubmit)="submit()">
      <h2 class="text-lg">Sign in</h2>

      @if (errorMessage) {
        <div class="form-error">{{ errorMessage }}</div>
      }

      <div>
        <label class="mb-1 block text-sm font-medium" for="user">Username</label>
        <input
          id="user"
          class="input"
          type="text"
          formControlName="user"
          placeholder="Username"
          (focus)="clearError()"
        />
        @if (loginForm.controls.user.invalid && loginForm.controls.user.touched) {
          <div class="field-error">Username is required.</div>
        }
      </div>

      <div>
        <label class="mb-1 block text-sm font-medium" for="pass">Password</label>
        <input
          id="pass"
          class="input"
          type="password"
          formControlName="pass"
          placeholder="Password"
          (focus)="clearError()"
        />
        @if (loginForm.controls.pass.invalid && loginForm.controls.pass.touched) {
          <div class="field-error">Password is required.</div>
        }
      </div>

      <button class="btn btn-primary mt-2" type="submit" [disabled]="loginForm.invalid">
        Login
      </button>
    </form>
  `,
  changeDetection: ChangeDetectionStrategy.Eager,
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
          // Clear invalid credentials
          this.loginForm.controls.user.setValue("");
          this.loginForm.controls.pass.setValue("");
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

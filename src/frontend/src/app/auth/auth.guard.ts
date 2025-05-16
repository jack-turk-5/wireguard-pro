import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { AuthService } from '../services/auth.service';

export const authGuard: CanActivateFn = (route, state) => {
  const authService = inject(AuthService);
  const router = inject(Router);
  const token = authService.token

  if (!token) {
    // If no token, redirect to login and deny activation.
    router.navigateByUrl('/login');
    return false;
  }
  else {
    return true;
  }
}
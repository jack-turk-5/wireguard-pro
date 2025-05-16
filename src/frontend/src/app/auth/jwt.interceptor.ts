import { HttpErrorResponse, HttpInterceptorFn, HttpRequest, HttpResponse } from "@angular/common/http";
import { inject } from "@angular/core";
import { Router } from "@angular/router";
import { catchError, of, throwError } from "rxjs";
import { AuthService } from "../services/auth.service";

export const JwtInterceptor: HttpInterceptorFn = (req, next) => {
  // If this is server-side, pass along dummy
  // TODO more research needed here
  if (!(req instanceof HttpRequest)) {
    return of(new HttpResponse({
      status: 204,
      statusText: 'No Content', 
      body: null
    }));
  }

  const authService = inject(AuthService);
  const router = inject(Router);

  const addHeaders = (request: HttpRequest<any>) => {
    let headers: { [header: string]: string } = {};
    if (!request.url.endsWith('/auth/login')) {
      const token = authService.token;
      if (token) {
        headers['Authorization'] = `Bearer ${token}`;
      } else {
        router.navigateByUrl('/login');
        throw new Error('No token found. Redirecting to login.');
      }
    }
    return request.clone({
      setHeaders: headers
    });
  };

  const handleError = (error: HttpErrorResponse) => {
    if (error.status === 401 || error.status === 403) {
      router.navigateByUrl('/login');
    }
    console.error('HTTP Error:', error.message);
    return throwError(() => new Error(error.message));
  };

  try {
    const clonedRequest = addHeaders(req);
    return next(clonedRequest).pipe(
      catchError(handleError)
    );
  } catch (error) {
    return throwError(() => error);
  }

};
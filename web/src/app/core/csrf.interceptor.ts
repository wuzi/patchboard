import { HttpErrorResponse, HttpInterceptorFn } from '@angular/common/http';
import { catchError, throwError } from 'rxjs';
import { ApiError } from './api-error';

/** Adds the CSRF header to writes and turns HTTP failures into ApiError. */
export const csrfInterceptor: HttpInterceptorFn = (req, next) => {
  const r = req.method === 'GET' || req.method === 'HEAD' ? req : req.clone({ setHeaders: { 'X-PB-CSRF': '1' } });
  return next(r).pipe(
    catchError((e: unknown) => {
      if (e instanceof HttpErrorResponse) {
        const body = e.error?.error;
        const code = e.status === 0 ? 'network' : (body?.code ?? 'generic');
        return throwError(() => new ApiError(e.status, code, body?.fields ?? {}));
      }
      return throwError(() => e);
    }),
  );
};

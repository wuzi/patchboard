import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { MeStore } from '../../core/me.store';

/** Admin pages need a team session; everyone else goes to the sign-in page. */
export const adminGuard: CanActivateFn = () =>
  inject(MeStore).isAdmin() ? true : inject(Router).createUrlTree(['/admin/login']);

import { provideHttpClient, withInterceptors } from '@angular/common/http';
import { ApplicationConfig, inject, provideAppInitializer, provideBrowserGlobalErrorListeners } from '@angular/core';
import { RouteReuseStrategy, provideRouter, withComponentInputBinding } from '@angular/router';
import { routes } from './app.routes';
import { BoardReuseStrategy } from './core/board-reuse.strategy';
import { BoardStore } from './core/board.store';
import { csrfInterceptor } from './core/csrf.interceptor';
import { MeStore } from './core/me.store';

export const appConfig: ApplicationConfig = {
  providers: [
    provideBrowserGlobalErrorListeners(),
    provideRouter(routes, withComponentInputBinding()),
    provideHttpClient(withInterceptors([csrfInterceptor])),
    { provide: RouteReuseStrategy, useClass: BoardReuseStrategy },
    provideAppInitializer(() => Promise.all([inject(BoardStore).load(), inject(MeStore).refresh()])),
  ],
};

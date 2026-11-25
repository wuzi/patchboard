import { Injectable, inject, signal } from '@angular/core';
import { ApiError } from './api-error';
import { I18n } from './i18n/i18n';

export type ToastMsg = { id: number; text: string; kind: 'info' | 'error' };

/** Short notices in the corner; errors are translated from their API code. */
@Injectable({ providedIn: 'root' })
export class Toast {
  private readonly i18n = inject(I18n);
  private nextId = 1;
  readonly items = signal<ToastMsg[]>([]);

  show(text: string, kind: 'info' | 'error' = 'info'): void {
    const id = this.nextId++;
    this.items.update((l) => [...l, { id, text, kind }]);
    setTimeout(() => this.dismiss(id), 4500);
  }

  error(e: unknown): void {
    const code = e instanceof ApiError ? e.code : 'generic';
    const key = `error.${code}`;
    this.show(this.i18n.has(key) ? this.i18n.t(key) : this.i18n.t('error.generic'), 'error');
  }

  dismiss(id: number): void {
    this.items.update((l) => l.filter((t) => t.id !== id));
  }
}

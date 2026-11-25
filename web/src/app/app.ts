import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { RouterOutlet } from '@angular/router';
import { Toast } from './core/toast';

@Component({
  selector: 'pb-root',
  imports: [RouterOutlet],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <router-outlet />
    <div class="toasts" role="status" aria-live="polite">
      @for (t of toast.items(); track t.id) {
        <button type="button" class="toast" [class.error]="t.kind === 'error'" (click)="toast.dismiss(t.id)">{{ t.text }}</button>
      }
    </div>
  `,
  styles: `
    .toasts { position: fixed; right: 16px; bottom: 16px; left: 16px; display: flex; flex-direction: column;
      align-items: flex-end; gap: 8px; z-index: 1000; pointer-events: none; }
    .toast { pointer-events: auto; max-width: 420px; padding: 10px 16px; border-radius: 12px; border: 0;
      background: var(--pb-text); color: var(--pb-bg); font: inherit; font-size: 14px; text-align: left;
      box-shadow: 0 8px 24px rgb(0 0 0 / .18); cursor: pointer; }
    .toast.error { background: #dc2626; color: #fff; }
  `,
})
export class App {
  protected readonly toast = inject(Toast);
}

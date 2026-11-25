import { ChangeDetectionStrategy, Component, inject, input, output, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { Api } from '../../core/api';
import { I18n } from '../../core/i18n/i18n';
import { Toast } from '../../core/toast';

/** Inline form to ban a guest (optionally their IP), used from posts, comments and the vote audit. */
@Component({
  selector: 'pb-ban-form',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <form class="ban" data-testid="ban-form" (submit)="$event.preventDefault(); ban()">
      <label class="check"><input type="checkbox" [checked]="includeIp()" (change)="includeIp.set($any($event.target).checked)" />{{ i18n.t('admin.banIp') }}</label>
      <p class="muted note">{{ i18n.t('admin.banIpNote') }}</p>
      <input class="input" maxlength="200" [placeholder]="i18n.t('admin.banReason')" [attr.aria-label]="i18n.t('admin.banReason')"
        [value]="reason()" (input)="reason.set($any($event.target).value)" />
      <div class="actions-end">
        <button type="button" class="btn ghost" (click)="cancelled.emit()">{{ i18n.t('common.cancel') }}</button>
        <button type="submit" class="btn danger" [disabled]="busy()">{{ i18n.t('admin.ban') }}</button>
      </div>
    </form>
  `,
  styles: `.ban { margin-top: 8px; padding: 12px; border-radius: 12px; background: var(--pb-column); } .note { margin: -8px 0 8px; font-size: 12px; }`,
})
export class BanForm {
  readonly guestId = input.required<string>();
  readonly done = output<void>();
  readonly cancelled = output<void>();
  protected readonly i18n = inject(I18n);
  private readonly api = inject(Api);
  private readonly toast = inject(Toast);
  protected readonly includeIp = signal(true);
  protected readonly reason = signal('');
  protected readonly busy = signal(false);

  protected async ban(): Promise<void> {
    this.busy.set(true);
    try {
      await firstValueFrom(this.api.ban({ guestId: this.guestId(), includeIp: this.includeIp(), reason: this.reason() }));
      this.toast.show(this.i18n.t('common.saved'));
      this.done.emit();
    } catch (e) {
      this.toast.error(e);
    } finally {
      this.busy.set(false);
    }
  }
}

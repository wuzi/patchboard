import { ChangeDetectionStrategy, Component, inject, input, signal } from '@angular/core';
import { Router, RouterLink } from '@angular/router';
import { firstValueFrom } from 'rxjs';
import { Api } from '../../core/api';
import { I18n } from '../../core/i18n/i18n';
import { MeStore } from '../../core/me.store';
import { Toast } from '../../core/toast';
import { fieldMessage, fieldsOf } from './field-errors';

@Component({
  selector: 'pb-invite-accept',
  imports: [RouterLink],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <main class="auth">
      <div class="card">
        <h1>{{ i18n.t('admin.inviteTitle') }}</h1>
        @if (valid() === null) {
          <p class="muted">{{ i18n.t('common.loading') }}</p>
        } @else if (!valid()) {
          <p>{{ i18n.t('admin.inviteInvalid') }}</p>
          <a class="back" routerLink="/">{{ i18n.t('admin.backToBoard') }}</a>
        } @else {
          <form (submit)="$event.preventDefault(); submit()">
            <label class="field"><span>{{ i18n.t('admin.username') }}</span>
              <input class="input" autocomplete="username" [value]="username()" (input)="username.set($any($event.target).value)" />
              <span class="field-error">{{ msg('username') }}</span></label>
            <label class="field"><span>{{ i18n.t('admin.displayName') }}</span>
              <input class="input" maxlength="40" [value]="displayName()" (input)="displayName.set($any($event.target).value)" />
              <span class="field-error">{{ msg('displayName') }}</span></label>
            <label class="field"><span>{{ i18n.t('admin.password') }}</span>
              <input class="input" type="password" autocomplete="new-password" minlength="10" [value]="password()" (input)="password.set($any($event.target).value)" />
              <span class="field-error">{{ msg('password') }}</span></label>
            <button type="submit" class="btn primary wide" [disabled]="busy()">{{ i18n.t('admin.createAccount') }}</button>
          </form>
        }
      </div>
    </main>
  `,
})
export class InviteAccept {
  readonly token = input.required<string>();
  protected readonly i18n = inject(I18n);
  private readonly api = inject(Api);
  private readonly me = inject(MeStore);
  private readonly router = inject(Router);
  private readonly toast = inject(Toast);
  protected readonly valid = signal<boolean | null>(null);
  protected readonly username = signal('');
  protected readonly displayName = signal('');
  protected readonly password = signal('');
  protected readonly busy = signal(false);
  private readonly errors = signal<Record<string, string>>({});

  ngOnInit(): void {
    firstValueFrom(this.api.checkInvite(this.token())).then(
      (r) => this.valid.set(r.valid),
      () => this.valid.set(false),
    );
  }

  protected msg(field: string): string {
    return fieldMessage(this.i18n, this.errors()[field]);
  }

  protected async submit(): Promise<void> {
    this.busy.set(true);
    this.errors.set({});
    try {
      const me = await firstValueFrom(
        this.api.acceptInvite(this.token(), { username: this.username(), displayName: this.displayName(), password: this.password() }),
      );
      this.me.set(me);
      this.router.navigateByUrl('/admin');
    } catch (e) {
      const f = fieldsOf(e);
      if (Object.keys(f).length) this.errors.set(f);
      else this.toast.error(e);
    } finally {
      this.busy.set(false);
    }
  }
}

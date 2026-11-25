import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { Router, RouterLink } from '@angular/router';
import { firstValueFrom } from 'rxjs';
import { Api } from '../../core/api';
import { ApiError } from '../../core/api-error';
import { I18n, type TKey } from '../../core/i18n/i18n';
import { MeStore } from '../../core/me.store';

@Component({
  selector: 'pb-admin-login',
  imports: [RouterLink],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <main class="auth">
      <form class="card" (submit)="$event.preventDefault(); submit()">
        <h1>{{ i18n.t('admin.loginTitle') }}</h1>
        <label class="field"><span>{{ i18n.t('admin.username') }}</span>
          <input class="input" autocomplete="username" [value]="username()" (input)="username.set($any($event.target).value)" /></label>
        <label class="field"><span>{{ i18n.t('admin.password') }}</span>
          <input class="input" type="password" autocomplete="current-password" [value]="password()" (input)="password.set($any($event.target).value)" /></label>
        @if (error()) {<p class="field-error" role="alert">{{ error() }}</p>}
        <button type="submit" class="btn primary wide" [disabled]="busy() || !username() || !password()">{{ i18n.t('admin.login') }}</button>
        <a class="back" routerLink="/">{{ i18n.t('admin.backToBoard') }}</a>
      </form>
    </main>
  `,
})
export class AdminLogin {
  protected readonly i18n = inject(I18n);
  private readonly api = inject(Api);
  private readonly me = inject(MeStore);
  private readonly router = inject(Router);
  protected readonly username = signal('');
  protected readonly password = signal('');
  protected readonly busy = signal(false);
  protected readonly error = signal('');

  constructor() {
    if (this.me.isAdmin()) this.router.navigateByUrl('/admin');
  }

  protected async submit(): Promise<void> {
    this.busy.set(true);
    this.error.set('');
    try {
      this.me.set(await firstValueFrom(this.api.adminLogin(this.username(), this.password())));
      this.router.navigateByUrl('/admin');
    } catch (e) {
      const code = e instanceof ApiError ? e.code : 'generic';
      this.error.set(this.i18n.has(`error.${code}`) ? this.i18n.t(`error.${code}` as TKey) : this.i18n.t('error.generic'));
    } finally {
      this.busy.set(false);
    }
  }
}

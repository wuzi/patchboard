import { ChangeDetectionStrategy, Component, ElementRef, HostListener, inject, linkedSignal, signal } from '@angular/core';
import { Router, RouterLink } from '@angular/router';
import { firstValueFrom } from 'rxjs';
import { Api } from '../../core/api';
import { I18n, type Lang } from '../../core/i18n/i18n';
import { MeStore } from '../../core/me.store';
import { Toast } from '../../core/toast';
import { Avatar } from '../../shared/avatar';
import { Icon } from '../../shared/icon';

@Component({
  selector: 'pb-profile-menu',
  imports: [RouterLink, Avatar, Icon],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @let m = me.me();
    <button type="button" class="trigger" [attr.aria-label]="i18n.t('profile.open')" [attr.aria-expanded]="open()" (click)="open.set(!open())">
      @if (m.admin) {<pb-avatar [seed]="'admin-' + m.admin.id" [size]="28" />}
      @else if (m.guest) {<pb-avatar [seed]="m.guest.id" [size]="28" />}
      @else {<pb-icon name="user" />}
    </button>
    @if (open()) {
      <div class="panel" role="dialog" [attr.aria-label]="i18n.t('profile.open')">
        @if (m.admin) {
          <p class="who"><strong>{{ m.admin.displayName }}</strong> <span class="team-badge">{{ i18n.t('author.team') }}</span></p>
          <div class="row">
            <a class="btn" routerLink="/admin" (click)="open.set(false)"><pb-icon name="settings" />{{ i18n.t('admin.panel') }}</a>
            <button type="button" class="btn ghost" (click)="logout()"><pb-icon name="logout" />{{ i18n.t('admin.logout') }}</button>
          </div>
          <hr />
        }
        <form (submit)="$event.preventDefault(); saveName()">
          <label class="field"><span>{{ i18n.t('profile.nameLabel') }}</span>
            <span class="inline">
              <input class="input" maxlength="40" [attr.aria-label]="i18n.t('profile.nameLabel')" [value]="name()" (input)="name.set($any($event.target).value)" />
              <button type="submit" class="btn" [disabled]="busy()">{{ i18n.t('common.save') }}</button>
            </span>
          </label>
        </form>
        @if (m.guest) {
          <label class="field"><span>{{ i18n.t('profile.recoveryLabel') }}</span>
            <span class="inline">
              <input class="input mono" readonly data-testid="recovery-link" [value]="m.guest.recoveryUrl" (focus)="$any($event.target).select()" />
              <button type="button" class="btn" [attr.aria-label]="i18n.t('common.copy')" (click)="copy(m.guest.recoveryUrl)"><pb-icon name="copy" /></button>
            </span>
          </label>
          <p class="help">{{ i18n.t('profile.recoveryHelp') }} <strong>{{ i18n.t('profile.recoveryWarning') }}</strong></p>
        } @else {
          <p class="help">{{ i18n.t('profile.noIdentity') }}</p>
        }
        <hr />
        <div class="lang" role="group" [attr.aria-label]="i18n.t('profile.language')">
          <span class="muted"><pb-icon name="globe" />{{ i18n.t('profile.language') }}</span>
          @for (l of langs; track l) {
            <button type="button" class="pill" [class.on]="i18n.lang() === l" [attr.aria-pressed]="i18n.lang() === l" (click)="setLang(l)">{{ l.toUpperCase() }}</button>
          }
        </div>
      </div>
    }
  `,
  styles: `
    :host { position: relative; display: inline-flex; }
    .trigger { display: inline-flex; align-items: center; justify-content: center; width: 36px; height: 36px; border: 0; border-radius: 10px;
      background: transparent; color: var(--pb-muted); cursor: pointer; }
    .trigger:hover { background: var(--pb-hover); }
    .panel { position: absolute; right: 0; top: calc(100% + 6px); z-index: 30; width: min(340px, calc(100vw - 32px)); padding: 16px;
      border-radius: 16px; border: 1px solid var(--pb-border); background: var(--pb-surface); box-shadow: var(--pb-dialog-shadow); }
    .who { margin: 0 0 10px; }
    .row { display: flex; gap: 8px; flex-wrap: wrap; }
    hr { border: 0; border-top: 1px solid var(--pb-border); margin: 14px 0; }
    .inline { display: flex; gap: 8px; }
    .mono { font-family: ui-monospace, monospace; font-size: 12px; }
    .help { margin: -6px 0 0; font-size: 12px; color: var(--pb-muted); }
    .lang { display: flex; align-items: center; gap: 6px; }
    .lang .muted { display: inline-flex; align-items: center; gap: 6px; margin-right: auto; font-size: 13px; }
    .pill { padding: 4px 10px; border-radius: 999px; border: 1px solid var(--pb-border); background: transparent; cursor: pointer; font-size: 13px; font-weight: 600; }
    .pill.on { background: var(--pb-strong); border-color: var(--pb-strong); color: var(--pb-on-strong); }
  `,
})
export class ProfileMenu {
  protected readonly i18n = inject(I18n);
  protected readonly me = inject(MeStore);
  private readonly api = inject(Api);
  private readonly toast = inject(Toast);
  private readonly router = inject(Router);
  private readonly host = inject(ElementRef<HTMLElement>);
  protected readonly open = signal(false);
  protected readonly busy = signal(false);
  protected readonly name = linkedSignal(() => this.me.me().guest?.name ?? '');
  protected readonly langs: Lang[] = ['pt', 'en'];

  @HostListener('document:click', ['$event'])
  protected outside(e: MouseEvent): void {
    if (this.open() && !this.host.nativeElement.contains(e.target as Node)) this.open.set(false);
  }

  @HostListener('document:keydown.escape')
  protected escape(): void {
    this.open.set(false);
  }

  protected async saveName(): Promise<void> {
    this.busy.set(true);
    try {
      this.me.set(await firstValueFrom(this.api.setName(this.name())));
      this.toast.show(this.i18n.t('common.saved'));
    } catch (e) {
      this.toast.error(e);
    } finally {
      this.busy.set(false);
    }
  }

  protected async copy(text: string): Promise<void> {
    try {
      await navigator.clipboard.writeText(text);
      this.toast.show(this.i18n.t('common.copied'));
    } catch {
      /* the field is selectable as a fallback */
    }
  }

  protected setLang(l: Lang): void {
    this.i18n.setLang(l);
  }

  protected async logout(): Promise<void> {
    try {
      await firstValueFrom(this.api.adminLogout());
    } finally {
      await this.me.refresh();
      this.open.set(false);
      this.router.navigateByUrl('/');
    }
  }
}

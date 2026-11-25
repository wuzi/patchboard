import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { Api } from '../../core/api';
import type { Invite, TeamMember } from '../../core/api.types';
import { I18n } from '../../core/i18n/i18n';
import { MeStore } from '../../core/me.store';
import { Toast } from '../../core/toast';
import { Avatar } from '../../shared/avatar';
import { Icon } from '../../shared/icon';
import { fieldMessage, fieldsOf } from './field-errors';

@Component({
  selector: 'pb-team-page',
  imports: [Avatar, Icon],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <h1 class="admin-title">{{ i18n.t('admin.navTeam') }}</h1>
    <div class="rows">
      @for (m of members(); track m.id) {
        <div class="row-item">
          <pb-avatar [seed]="'admin-' + m.id" [size]="28" />
          <span class="name"><strong>{{ m.displayName }}</strong> <span class="muted">&#64;{{ m.username }}</span>
            @if (m.id === me.me().admin?.id) {<span class="muted"> ({{ i18n.t('admin.you') }})</span>}</span>
          @if (m.disabled) {<span class="chip">{{ i18n.t('admin.disabled') }}</span>}
          @else if (m.id !== me.me().admin?.id) {<button type="button" class="btn ghost danger" (click)="disable(m)">{{ i18n.t('admin.disable') }}</button>}
        </div>
      }
    </div>
    <section class="panel-card invite">
      <button type="button" class="btn primary" (click)="invite()"><pb-icon name="plus" />{{ i18n.t('admin.invite') }}</button>
      @if (inviteLink(); as inv) {
        <div class="inline"><input class="input mono" readonly data-testid="invite-url" [value]="inv.url" (focus)="$any($event.target).select()" />
          <button type="button" class="btn" [attr.aria-label]="i18n.t('common.copy')" (click)="copy(inv.url)"><pb-icon name="copy" /></button></div>
        <p class="muted help">{{ i18n.t('admin.inviteHelp', { date: i18n.date(inv.expiresAt, 'long') }) }}</p>
      }
    </section>
    <form class="panel-card" (submit)="$event.preventDefault(); saveMe()">
      <h2>{{ i18n.t('admin.myProfile') }}</h2>
      <label class="field"><span>{{ i18n.t('admin.displayName') }}</span>
        <input class="input" maxlength="40" [value]="displayName()" (input)="displayName.set($any($event.target).value)" /><span class="field-error">{{ msg('displayName') }}</span></label>
      <div class="grid-2">
        <label class="field"><span>{{ i18n.t('admin.currentPassword') }}</span>
          <input class="input" type="password" autocomplete="current-password" [value]="current()" (input)="current.set($any($event.target).value)" /><span class="field-error">{{ msg('currentPassword') }}</span></label>
        <label class="field"><span>{{ i18n.t('admin.newPassword') }}</span>
          <input class="input" type="password" autocomplete="new-password" [value]="next()" (input)="next.set($any($event.target).value)" /><span class="field-error">{{ msg('newPassword') }}</span></label>
      </div>
      <div class="actions-end"><button type="submit" class="btn primary">{{ i18n.t('common.save') }}</button></div>
    </form>
  `,
  styles: `
    .name { flex: 1; min-width: 0; }
    .invite { margin-top: 20px; display: flex; flex-direction: column; gap: 10px; align-items: flex-start; }
    .inline { display: flex; gap: 8px; width: 100%; }
    .mono { font-family: ui-monospace, monospace; font-size: 12px; }
    .help { margin: 0; font-size: 13px; }
  `,
})
export class TeamPage {
  protected readonly i18n = inject(I18n);
  protected readonly me = inject(MeStore);
  private readonly api = inject(Api);
  private readonly toast = inject(Toast);
  protected readonly members = signal<TeamMember[]>([]);
  protected readonly inviteLink = signal<Invite | null>(null);
  protected readonly displayName = signal(this.me.me().admin?.displayName ?? '');
  protected readonly current = signal('');
  protected readonly next = signal('');
  private readonly errors = signal<Record<string, string>>({});

  constructor() {
    this.load();
  }

  private async load(): Promise<void> {
    try {
      this.members.set(await firstValueFrom(this.api.team()));
    } catch (e) {
      this.toast.error(e);
    }
  }

  protected msg(f: string): string {
    return fieldMessage(this.i18n, this.errors()[f]);
  }

  protected async invite(): Promise<void> {
    try {
      this.inviteLink.set(await firstValueFrom(this.api.createInvite()));
    } catch (e) {
      this.toast.error(e);
    }
  }

  protected async copy(text: string): Promise<void> {
    try {
      await navigator.clipboard.writeText(text);
      this.toast.show(this.i18n.t('common.copied'));
    } catch {
      /* selectable field is the fallback */
    }
  }

  protected async disable(m: TeamMember): Promise<void> {
    if (!confirm(`${this.i18n.t('admin.disable')} ${m.displayName}?`)) return;
    try {
      await firstValueFrom(this.api.disableMember(m.id));
      await this.load();
    } catch (e) {
      this.toast.error(e);
    }
  }

  protected async saveMe(): Promise<void> {
    this.errors.set({});
    const body: { displayName?: string; currentPassword?: string; newPassword?: string } = { displayName: this.displayName() };
    if (this.next()) {
      body.currentPassword = this.current();
      body.newPassword = this.next();
    }
    try {
      this.me.set(await firstValueFrom(this.api.updateAdminMe(body)));
      this.current.set('');
      this.next.set('');
      this.toast.show(this.i18n.t('common.saved'));
      await this.load();
    } catch (e) {
      const f = fieldsOf(e);
      if (Object.keys(f).length) this.errors.set(f);
      else this.toast.error(e);
    }
  }
}

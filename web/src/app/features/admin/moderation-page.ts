import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { firstValueFrom } from 'rxjs';
import { Api } from '../../core/api';
import type { Ban, PostCard, Trash } from '../../core/api.types';
import { BoardEvents } from '../../core/board-events';
import { I18n, type TKey } from '../../core/i18n/i18n';
import { postUrl } from '../../core/route-utils';
import { Toast } from '../../core/toast';

type Tab = 'pending' | 'trash' | 'bans';

@Component({
  selector: 'pb-moderation-page',
  imports: [RouterLink],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <h1 class="admin-title">{{ i18n.t('admin.navModeration') }}</h1>
    <div class="tabs" role="tablist">
      @for (t of tabs; track t.id) {
        <button type="button" role="tab" [attr.aria-selected]="tab() === t.id" [class.on]="tab() === t.id" (click)="tab.set(t.id)">{{ i18n.t(t.label) }}</button>
      }
    </div>
    @switch (tab()) {
      @case ('pending') {
        @for (p of pending(); track p.id) {
          <div class="row-item" data-testid="pending-row">
            <a class="grow" [routerLink]="url(p)">{{ p.title }}</a><span class="muted">{{ i18n.date(p.createdAt) }}</span>
            <button type="button" class="btn primary" (click)="approve(p)">{{ i18n.t('admin.approve') }}</button>
            <button type="button" class="btn ghost danger" (click)="reject(p)">{{ i18n.t('common.delete') }}</button>
          </div>
        } @empty {<p class="empty">{{ i18n.t('admin.nothingPending') }}</p>}
      }
      @case ('trash') {
        @for (p of trash().posts; track p.id) {
          <div class="row-item"><span class="grow"><strong>{{ p.title }}</strong> <span class="muted">· {{ i18n.t(deletedBy(p.deletedBy)) }} · {{ i18n.date(p.deletedAt) }}</span></span>
            <button type="button" class="btn" (click)="restorePost(p.id)">{{ i18n.t('common.restore') }}</button></div>
        }
        @for (c of trash().comments; track c.id) {
          <div class="row-item"><span class="grow">“{{ c.excerpt }}” <span class="muted">· {{ c.postTitle }} · {{ i18n.t(deletedBy(c.deletedBy)) }}</span></span>
            <button type="button" class="btn" (click)="restoreComment(c.id)">{{ i18n.t('common.restore') }}</button></div>
        }
        @if (!trash().posts.length && !trash().comments.length) {<p class="empty">{{ i18n.t('admin.trashEmpty') }}</p>}
      }
      @case ('bans') {
        @for (b of bans(); track b.id) {
          <div class="row-item"><span class="grow"><strong>{{ b.guestName ?? i18n.t('author.anonymous') }}</strong>
            @if (b.ipHash) {<span class="muted mono"> · IP {{ b.ipHash }}</span>} @if (b.reason) {<span class="muted"> · {{ b.reason }}</span>}</span>
            <button type="button" class="btn" (click)="unban(b)">{{ i18n.t('admin.unban') }}</button></div>
        } @empty {<p class="empty">{{ i18n.t('admin.noBans') }}</p>}
      }
    }
  `,
  styles: `
    .tabs { display: flex; gap: 4px; margin-bottom: 14px; }
    .tabs button { padding: 6px 12px; border: 0; border-radius: 999px; background: transparent; color: var(--pb-muted); font-weight: 500; cursor: pointer; }
    .tabs button.on { background: var(--pb-chip); color: var(--pb-text); }
    .row-item { margin-bottom: 8px; }
    .grow { flex: 1; min-width: 0; }
    .mono { font-family: ui-monospace, monospace; font-size: 12px; }
  `,
})
export class ModerationPage {
  protected readonly i18n = inject(I18n);
  private readonly api = inject(Api);
  private readonly toast = inject(Toast);
  private readonly events = inject(BoardEvents);
  protected readonly tabs: { id: Tab; label: TKey }[] = [
    { id: 'pending', label: 'admin.pending' },
    { id: 'trash', label: 'admin.trash' },
    { id: 'bans', label: 'admin.bans' },
  ];
  protected readonly tab = signal<Tab>('pending');
  protected readonly pending = signal<PostCard[]>([]);
  protected readonly trash = signal<Trash>({ posts: [], comments: [] });
  protected readonly bans = signal<Ban[]>([]);
  protected readonly url = postUrl;

  constructor() {
    this.load();
  }

  protected deletedBy(by: 'author' | 'admin'): TKey {
    return `admin.deletedBy_${by}`;
  }

  private async load(): Promise<void> {
    try {
      const [p, t, b] = await Promise.all([firstValueFrom(this.api.pending()), firstValueFrom(this.api.trash()), firstValueFrom(this.api.bans())]);
      this.pending.set(p);
      this.trash.set(t);
      this.bans.set(b);
    } catch (e) {
      this.toast.error(e);
    }
  }

  private async run(op: Promise<unknown>): Promise<void> {
    try {
      await op;
      this.events.changed();
    } catch (e) {
      this.toast.error(e);
    }
    await this.load();
  }

  protected approve(p: PostCard): void {
    this.run(firstValueFrom(this.api.approve(p.id)));
  }

  protected reject(p: PostCard): void {
    if (confirm(this.i18n.t('post.confirmDelete'))) this.run(firstValueFrom(this.api.deletePost(p.id)));
  }

  protected restorePost(id: number): void {
    this.run(firstValueFrom(this.api.restorePost(id)));
  }

  protected restoreComment(id: number): void {
    this.run(firstValueFrom(this.api.restoreComment(id)));
  }

  protected unban(b: Ban): void {
    this.run(firstValueFrom(this.api.unban(b.id)));
  }
}

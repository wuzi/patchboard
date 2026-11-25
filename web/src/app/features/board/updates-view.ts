import { ChangeDetectionStrategy, Component, effect, inject, signal, untracked } from '@angular/core';
import { RouterLink } from '@angular/router';
import { firstValueFrom } from 'rxjs';
import { Api } from '../../core/api';
import type { UpdateItem } from '../../core/api.types';
import { BoardEvents } from '../../core/board-events';
import { BoardStore } from '../../core/board.store';
import { I18n } from '../../core/i18n/i18n';
import { postUrl } from '../../core/route-utils';
import { Toast } from '../../core/toast';

@Component({
  selector: 'pb-updates-view',
  imports: [RouterLink],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (loading() && items().length === 0) {
      <p class="empty">{{ i18n.t('common.loading') }}</p>
    } @else if (items().length === 0) {
      <p class="empty">{{ i18n.t('updates.empty') }}</p>
    }
    <div class="list">
      @for (u of items(); track u.id) {
        <article class="update">
          <p class="date">{{ i18n.date(u.publishedAt, 'long') }}</p>
          <h2>{{ u.title }}</h2>
          <div class="md body" [innerHTML]="u.bodyHtml"></div>
          @if (u.posts.length) {
            <div class="posts">
              @for (p of u.posts; track p.id) {
                @let s = store.status(p.statusId);
                <a class="post" [routerLink]="url(p)">
                  @if (s) {<span class="status"><span class="dot" [style.background]="s.color"></span>{{ i18n.name(s.name) }}</span>}
                  <span class="title">{{ p.title }}</span>
                </a>
              }
            </div>
          }
        </article>
      }
    </div>
    @if (next() !== null) {
      <div class="more"><button type="button" class="btn" (click)="load(next()!)">{{ i18n.t('common.loadMore') }}</button></div>
    }
  `,
  styles: `
    .list { display: flex; flex-direction: column; gap: 14px; }
    .update { padding: 22px 24px; border-radius: 18px; border: 1px solid var(--pb-border); background: var(--pb-surface); }
    .date { margin: 0; color: var(--pb-muted); font-size: 14px; }
    h2 { margin: 4px 0 8px; font-size: 20px; font-weight: 600; }
    .body { color: var(--pb-muted); font-size: 16px; }
    .posts { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 16px; }
    .post { display: inline-flex; align-items: center; gap: 10px; padding: 4px 14px 4px 4px; border-radius: 999px;
      border: 1px solid var(--pb-border); text-decoration: none; font-size: 15px; }
    .post:hover { background: var(--pb-hover); }
    .status { display: inline-flex; align-items: center; gap: 6px; padding: 4px 10px; border-radius: 999px; background: var(--pb-chip);
      color: var(--pb-muted); font-size: 13px; }
    .dot { width: 8px; height: 8px; border-radius: 50%; }
    .more { display: flex; justify-content: center; margin-top: 16px; }
  `,
})
export class UpdatesView {
  protected readonly i18n = inject(I18n);
  protected readonly store = inject(BoardStore);
  private readonly api = inject(Api);
  private readonly toast = inject(Toast);
  private readonly events = inject(BoardEvents);
  protected readonly items = signal<UpdateItem[]>([]);
  protected readonly next = signal<number | null>(null);
  protected readonly loading = signal(true);
  protected readonly url = postUrl;

  constructor() {
    effect(() => {
      this.events.revision();
      untracked(() => this.load(0));
    });
  }

  async load(offset: number): Promise<void> {
    this.loading.set(true);
    try {
      const page = await firstValueFrom(this.api.updates(offset));
      this.items.update((l) => (offset === 0 ? page.updates : [...l, ...page.updates]));
      this.next.set(page.nextOffset);
    } catch (e) {
      this.toast.error(e);
    } finally {
      this.loading.set(false);
    }
  }
}

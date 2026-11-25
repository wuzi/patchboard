import { ChangeDetectionStrategy, Component, computed, effect, inject, signal, untracked } from '@angular/core';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { ActivatedRoute, Router } from '@angular/router';
import { firstValueFrom, map } from 'rxjs';
import { Api } from '../../core/api';
import type { PostCard as Card, Sort } from '../../core/api.types';
import { BoardEvents } from '../../core/board-events';
import { I18n, type TKey } from '../../core/i18n/i18n';
import { Toast } from '../../core/toast';
import { PostCard } from '../../shared/post-card';

export type ListQuery = { sort: Sort; status: number | null; tag: number | null; q: string };

const num = (v: string | null): number | null => (v && /^\d+$/.test(v) ? Number(v) : null);

/** Reads the shared board filters (sort, status, tag, q) from the URL. */
export function boardQuery(route: ActivatedRoute) {
  return toSignal(
    route.queryParamMap.pipe(
      map((q): ListQuery => {
        const sort = q.get('sort');
        return {
          sort: sort === 'top' || sort === 'new' ? sort : 'trending',
          status: num(q.get('status')),
          tag: num(q.get('tag')),
          q: (q.get('q') ?? '').trim(),
        };
      }),
    ),
    { initialValue: { sort: 'trending', status: null, tag: null, q: '' } as ListQuery },
  );
}

@Component({
  selector: 'pb-list-view',
  imports: [PostCard],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="sorts" role="tablist" [attr.aria-label]="i18n.t('nav.list')">
      @for (s of sorts; track s.value) {
        <button type="button" role="tab" class="sort" [class.active]="query().sort === s.value"
          [attr.aria-selected]="query().sort === s.value" (click)="setSort(s.value)">{{ i18n.t(s.label) }}</button>
      }
    </div>
    @if (loading() && posts().length === 0) {
      <p class="empty">{{ i18n.t('common.loading') }}</p>
    } @else if (posts().length === 0) {
      <p class="empty">{{ i18n.t('common.empty') }}</p>
    } @else {
      <div class="posts">
        @for (p of posts(); track p.id) {<pb-post-card [post]="p" [showStatus]="true" />}
      </div>
      @if (next() !== null) {
        <div class="more"><button type="button" class="btn" [disabled]="loading()" (click)="loadMore()">{{ i18n.t('common.loadMore') }}</button></div>
      }
    }
  `,
  styles: `
    .sorts { display: flex; gap: 4px; margin-bottom: 14px; }
    .sort { padding: 6px 12px; border-radius: 999px; border: 0; background: transparent; color: var(--pb-muted);
      font-size: 14px; font-weight: 500; cursor: pointer; }
    .sort:hover { background: var(--pb-hover); color: var(--pb-text); }
    .sort.active { background: var(--pb-chip); color: var(--pb-text); }
    .posts { display: flex; flex-direction: column; gap: 10px; }
    .more { display: flex; justify-content: center; margin-top: 16px; }
  `,
})
export class ListView {
  protected readonly i18n = inject(I18n);
  private readonly api = inject(Api);
  private readonly router = inject(Router);
  private readonly route = inject(ActivatedRoute);
  private readonly toast = inject(Toast);
  private readonly events = inject(BoardEvents);

  protected readonly sorts: { value: Sort; label: TKey }[] = [
    { value: 'trending', label: 'sort.trending' },
    { value: 'top', label: 'sort.top' },
    { value: 'new', label: 'sort.new' },
  ];
  protected readonly query = boardQuery(this.route);
  private readonly queryKey = computed(() => JSON.stringify(this.query()));
  protected readonly posts = signal<Card[]>([]);
  protected readonly next = signal<number | null>(null);
  protected readonly loading = signal(true);
  private seq = 0;

  constructor() {
    effect(() => {
      this.queryKey();
      this.events.revision();
      untracked(() => this.load(0));
    });
    this.events.votes.pipe(takeUntilDestroyed()).subscribe(({ postId, state }) =>
      this.posts.update((l) => l.map((p) => (p.id === postId ? { ...p, voteCount: state.voteCount, voted: state.voted } : p))),
    );
  }

  private async load(offset: number): Promise<void> {
    const mine = ++this.seq;
    this.loading.set(true);
    try {
      const page = await firstValueFrom(this.api.listPosts({ ...this.query(), offset }));
      if (mine !== this.seq) return;
      this.posts.update((l) => (offset === 0 ? page.posts : [...l, ...page.posts]));
      this.next.set(page.nextOffset);
    } catch (e) {
      if (mine === this.seq) this.toast.error(e);
    } finally {
      if (mine === this.seq) this.loading.set(false);
    }
  }

  protected loadMore(): void {
    const n = this.next();
    if (n !== null) this.load(n);
  }

  protected setSort(sort: Sort): void {
    this.router.navigate([], { queryParams: { sort: sort === 'trending' ? null : sort }, queryParamsHandling: 'merge' });
  }
}

import { CdkDrag, CdkDragDrop, CdkDropList, CdkDropListGroup } from '@angular/cdk/drag-drop';
import { ChangeDetectionStrategy, Component, computed, effect, inject, output, signal, untracked } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ActivatedRoute } from '@angular/router';
import { firstValueFrom } from 'rxjs';
import { Api } from '../../core/api';
import type { PostCard as Card, Roadmap, Status } from '../../core/api.types';
import { BoardEvents } from '../../core/board-events';
import { BoardStore } from '../../core/board.store';
import { I18n } from '../../core/i18n/i18n';
import { MeStore } from '../../core/me.store';
import { Toast } from '../../core/toast';
import { Icon } from '../../shared/icon';
import { PostCard } from '../../shared/post-card';
import { boardQuery } from './list-view';

export type RoadmapDrop = { post: Card; toStatusId: number; revert: () => void };

type Column = { status: Status; total: number; posts: Card[] };

@Component({
  selector: 'pb-roadmap-view',
  imports: [CdkDropListGroup, CdkDropList, CdkDrag, PostCard, Icon],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="scroller">
      <div class="columns" cdkDropListGroup>
        @for (col of columns(); track col.status.id) {
          <section class="column" role="region" [attr.aria-label]="i18n.name(col.status.name)">
            <header class="col-head">
              <span class="dot" [style.background]="col.status.color"></span>
              <h2>{{ i18n.name(col.status.name) }}</h2>
              <span class="count" data-testid="column-count">{{ col.total }}</span>
            </header>
            <div class="cards" cdkDropList [id]="'col-' + col.status.id" [cdkDropListData]="col"
              [cdkDropListDisabled]="!me.isAdmin()" (cdkDropListDropped)="drop($event)">
              @for (p of col.posts; track p.id) {
                <div class="drag" cdkDrag [cdkDragData]="p" [cdkDragDisabled]="!me.isAdmin()"><pb-post-card [post]="p" /></div>
              } @empty {
                <p class="empty">{{ i18n.t('common.empty') }}</p>
              }
            </div>
            @if (col.total > col.posts.length) {<p class="more">{{ i18n.t('roadmap.more', { n: col.total - col.posts.length }) }}</p>}
            @if (col.status.isDefault) {
              <button type="button" class="add" (click)="events.newPostOpen.set(true)"><pb-icon name="plus" />{{ i18n.t('post.add') }}</button>
            }
          </section>
        }
      </div>
    </div>
  `,
  styles: `
    .scroller { overflow-x: auto; padding: 0 16px 8px; scroll-snap-type: x proximity; }
    .columns { display: flex; gap: 14px; width: max-content; margin: 0 auto; align-items: flex-start; }
    .column { width: 272px; flex: none; padding: 12px; border-radius: 18px; background: var(--pb-column); min-height: 55vh;
      scroll-snap-align: start; }
    .col-head { display: flex; align-items: center; gap: 8px; padding: 2px 6px 12px; }
    .col-head h2 { margin: 0; font-size: 15px; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .dot { width: 9px; height: 9px; border-radius: 50%; flex: none; }
    .count { margin-left: auto; color: var(--pb-muted); font-size: 13px; font-variant-numeric: tabular-nums; }
    .cards { display: flex; flex-direction: column; gap: 10px; min-height: 40px; }
    .drag { border-radius: var(--pb-radius); }
    .cdk-drag-preview { box-shadow: var(--pb-dialog-shadow); border-radius: var(--pb-radius); }
    .cdk-drag-placeholder { opacity: .35; }
    .cdk-drag-animating, .cards.cdk-drop-list-dragging .drag:not(.cdk-drag-placeholder) { transition: transform 180ms ease; }
    .empty { padding: 12px 0; }
    .more { margin: 10px 4px 0; color: var(--pb-muted); font-size: 13px; }
    .add { display: flex; align-items: center; gap: 8px; width: 100%; margin-top: 10px; padding: 10px 12px; border-radius: 12px;
      border: 1px dashed var(--pb-border); background: transparent; color: var(--pb-muted); cursor: pointer; font-weight: 500; }
    .add:hover { background: var(--pb-surface); color: var(--pb-text); }
    @media (max-width: 600px) { .column { width: 82vw; } }
  `,
})
export class RoadmapView {
  readonly moved = output<RoadmapDrop>();

  protected readonly i18n = inject(I18n);
  protected readonly me = inject(MeStore);
  protected readonly events = inject(BoardEvents);
  private readonly store = inject(BoardStore);
  private readonly api = inject(Api);
  private readonly toast = inject(Toast);
  private readonly route = inject(ActivatedRoute);
  private readonly query = boardQuery(this.route);
  private readonly key = computed(() => JSON.stringify({ tag: this.query().tag, q: this.query().q }));
  private readonly data = signal<Roadmap>({ columns: [] });
  private seq = 0;

  protected readonly columns = computed<Column[]>(() =>
    this.data().columns.flatMap((c) => {
      const status = this.store.status(c.statusId);
      return status ? [{ status, total: c.total, posts: c.posts }] : [];
    }),
  );

  constructor() {
    effect(() => {
      this.key();
      this.events.revision();
      untracked(() => this.load());
    });
    this.events.votes.pipe(takeUntilDestroyed()).subscribe(({ postId, state }) =>
      this.data.update((d) => ({
        columns: d.columns.map((c) => ({
          ...c,
          posts: c.posts.map((p) => (p.id === postId ? { ...p, voteCount: state.voteCount, voted: state.voted } : p)),
        })),
      })),
    );
  }

  private async load(): Promise<void> {
    const mine = ++this.seq;
    try {
      const rm = await firstValueFrom(this.api.roadmap({ tag: this.query().tag, q: this.query().q }));
      if (mine === this.seq) this.data.set(rm);
    } catch (e) {
      if (mine === this.seq) this.toast.error(e);
    }
  }

  protected drop(e: CdkDragDrop<Column, Column, Card>): void {
    if (e.previousContainer === e.container) return;
    const before = this.data();
    const post = e.item.data;
    const to = e.container.data.status.id;
    this.data.update((d) => ({
      columns: d.columns.map((c) =>
        c.statusId === to
          ? { ...c, total: c.total + 1, posts: [post, ...c.posts] }
          : c.posts.some((p) => p.id === post.id)
            ? { ...c, total: c.total - 1, posts: c.posts.filter((p) => p.id !== post.id) }
            : c,
      ),
    }));
    this.moved.emit({ post, toStatusId: to, revert: () => this.data.set(before) });
  }
}

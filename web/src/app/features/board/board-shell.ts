import { ChangeDetectionStrategy, Component, ElementRef, computed, effect, inject, signal, viewChild } from '@angular/core';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { Title } from '@angular/platform-browser';
import { ActivatedRoute, NavigationEnd, Router, RouterLink } from '@angular/router';
import { debounceTime, filter, map, Subject } from 'rxjs';
import { BoardEvents } from '../../core/board-events';
import { BoardStore } from '../../core/board.store';
import { I18n } from '../../core/i18n/i18n';
import { Toast } from '../../core/toast';
import { NavState, type BoardView } from '../../core/nav-state';
import { Icon } from '../../shared/icon';
import { NewPostDialog } from '../post/new-post-dialog';
import { PostModal } from '../post/post-modal';
import { boardQuery, ListView } from './list-view';
import { RoadmapView, type RoadmapDrop } from './roadmap-view';
import { StatusChangeDialog } from '../admin/status-change-dialog';
import { UpdatesView } from './updates-view';
import { ProfileMenu } from '../profile/profile-menu';

type View = BoardView | 'post';

@Component({
  selector: 'pb-board-shell',
  imports: [RouterLink, Icon, ListView, RoadmapView, UpdatesView, ProfileMenu, PostModal, NewPostDialog, StatusChangeDialog],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './board-shell.html',
  styleUrl: './board-shell.scss',
})
export class BoardShell {
  protected readonly i18n = inject(I18n);
  protected readonly store = inject(BoardStore);
  protected readonly events = inject(BoardEvents);
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly nav = inject(NavState);
  private readonly title = inject(Title);
  private readonly toast = inject(Toast);

  protected readonly view = toSignal(this.route.data.pipe(map((d) => d['view'] as View)), { initialValue: 'list' as View });
  protected readonly idSlug = toSignal(this.route.paramMap.pipe(map((p) => p.get('idSlug'))), { initialValue: null });
  /** The view drawn on the page: for /p/:idSlug it is whatever was open before the modal. */
  protected readonly background = computed<BoardView>(() => {
    const v = this.view();
    return v === 'post' ? this.nav.lastView() : v;
  });
  protected readonly query = boardQuery(this.route);
  protected readonly hasFilters = computed(() => this.query().status !== null || this.query().tag !== null);

  protected readonly drop = signal<RoadmapDrop | null>(null);
  protected readonly searchOpen = signal(false);
  protected readonly filterOpen = signal(false);
  private readonly searchInput = viewChild<ElementRef<HTMLInputElement>>('search');
  private readonly typed = new Subject<string>();

  protected readonly tabs: { view: BoardView; link: string; icon: string; label: 'nav.list' | 'nav.roadmap' | 'nav.updates' }[] = [
    { view: 'list', link: '/', icon: 'list', label: 'nav.list' },
    { view: 'roadmap', link: '/roadmap', icon: 'roadmap', label: 'nav.roadmap' },
    { view: 'updates', link: '/updates', icon: 'updates', label: 'nav.updates' },
  ];

  constructor() {
    this.router.events
      .pipe(filter((e): e is NavigationEnd => e instanceof NavigationEnd), takeUntilDestroyed())
      .subscribe(() => this.remember());
    this.remember();
    this.typed.pipe(debounceTime(300), takeUntilDestroyed()).subscribe((q) =>
      this.router.navigate([], { queryParams: { q: q.trim() || null }, queryParamsHandling: 'merge', replaceUrl: true }),
    );
    if (this.query().q) this.searchOpen.set(true);
    this.announceRecovery();
    effect(() => {
      const b = this.store.board();
      if (b && this.view() !== 'post') this.title.setTitle(this.i18n.name(b.title));
    });
  }

  /** /r/<token> redirects to /?recovered=1|0; tell the player and drop the param. */
  private announceRecovery(): void {
    const r = this.route.snapshot.queryParamMap.get('recovered');
    if (r === null) return;
    if (r === '1') this.toast.show(this.i18n.t('profile.recovered'));
    else this.toast.show(this.i18n.t('profile.recoverFailed'), 'error');
    this.router.navigate([], { queryParams: { recovered: null }, queryParamsHandling: 'merge', replaceUrl: true });
  }

  private remember(): void {
    const v = this.view();
    if (v !== 'post') {
      this.nav.lastView.set(v);
      this.nav.lastUrl.set(this.router.url);
    }
  }

  protected dropDone(): void {
    this.drop.set(null);
    this.events.changed();
  }

  protected dropCancelled(d: RoadmapDrop): void {
    d.revert();
    this.drop.set(null);
  }

  protected toggleSearch(): void {
    const open = !this.searchOpen();
    this.searchOpen.set(open);
    if (open) setTimeout(() => this.searchInput()?.nativeElement.focus());
    else if (this.query().q) this.typed.next('');
  }

  protected onSearch(value: string): void {
    this.typed.next(value);
  }

  protected setFilter(name: 'status' | 'tag', value: string): void {
    this.router.navigate([], { queryParams: { [name]: value || null }, queryParamsHandling: 'merge' });
  }

  protected clearFilters(): void {
    this.router.navigate([], { queryParams: { status: null, tag: null }, queryParamsHandling: 'merge' });
    this.filterOpen.set(false);
  }
}

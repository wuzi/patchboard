import { ChangeDetectionStrategy, Component, ElementRef, afterNextRender, computed, inject, linkedSignal, output, signal, viewChild } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Router, RouterLink } from '@angular/router';
import { debounceTime, distinctUntilChanged, firstValueFrom, of, Subject, switchMap, catchError } from 'rxjs';
import { Api } from '../../core/api';
import { ApiError } from '../../core/api-error';
import type { Image, Similar } from '../../core/api.types';
import { BoardEvents } from '../../core/board-events';
import { BoardStore } from '../../core/board.store';
import { I18n, type TKey } from '../../core/i18n/i18n';
import { MeStore } from '../../core/me.store';
import { postUrl } from '../../core/route-utils';
import { Toast } from '../../core/toast';
import { Icon } from '../../shared/icon';
import { ImagePicker } from '../../shared/image-picker';
import { MarkdownEditor } from '../../shared/markdown-editor/markdown-editor';
import { StatusChip } from '../../shared/status-chip';

@Component({
  selector: 'pb-new-post-dialog',
  imports: [RouterLink, Icon, ImagePicker, MarkdownEditor, StatusChip],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <dialog #dialog class="pb-dialog" (cancel)="$event.preventDefault(); closed.emit()" [attr.aria-label]="i18n.t('post.newTitle')">
      <form class="wrap" (submit)="$event.preventDefault(); submit()">
        <header class="top"><h2>{{ i18n.t('post.newTitle') }}</h2>
          <button type="button" class="icon-btn" [attr.aria-label]="i18n.t('common.close')" (click)="closed.emit()"><pb-icon name="close" /></button></header>

        <label class="field"><span>{{ i18n.t('post.titleLabel') }}</span>
          <input #titleInput class="input" [class.invalid]="errors()['title']" maxlength="120" [value]="title()"
            (input)="setTitle($any($event.target).value)" autocomplete="off" />
          @if (errors()['title']; as r) {<span class="field-error">{{ reason(r) }}</span>}
        </label>

        @if (similar().length) {
          <div class="similar">
            <p class="muted">{{ i18n.t('post.similarTitle') }}</p>
            <ul>
              @for (s of similar(); track s.id) {
                <li><a [routerLink]="url(s)" (click)="closed.emit()">{{ s.title }}</a>
                  <span class="votes">▲ {{ s.voteCount }}</span><pb-status-chip [statusId]="s.statusId" /></li>
              }
            </ul>
          </div>
        }

        <div class="field"><span>{{ i18n.t('post.bodyLabel') }}</span>
          <pb-markdown-editor [(value)]="body" [placeholder]="i18n.t('post.bodyPlaceholder')" [label]="i18n.t('post.bodyLabel')" [rows]="5" />
          @if (errors()['body']; as r) {<span class="field-error">{{ reason(r) }}</span>}
        </div>

        <div class="cols">
          <label class="field"><span>{{ i18n.t('post.tagLabel') }}</span>
            <select class="input" (change)="tagId.set($any($event.target).value ? +$any($event.target).value : null)">
              <option value="">{{ i18n.t('post.noTag') }}</option>
              @for (t of store.tags(); track t.id) {<option [value]="t.id" [selected]="t.id === tagId()">{{ i18n.name(t.name) }}</option>}
            </select>
          </label>
          @if (!me.isAdmin()) {
            <label class="field"><span>{{ i18n.t('profile.nameLabel') }}</span>
              <input class="input" maxlength="40" [placeholder]="i18n.t('comment.namePlaceholder')" [value]="name()" (input)="name.set($any($event.target).value)" />
            </label>
          }
        </div>

        @if (store.board()?.uploadsEnabled || me.isAdmin()) {
          <div class="field"><pb-image-picker [(images)]="images" />
            @if (errors()['imageIds']; as r) {<span class="field-error">{{ reason(r) }}</span>}</div>
        }

        <footer class="actions">
          <button type="button" class="btn ghost" (click)="closed.emit()">{{ i18n.t('common.cancel') }}</button>
          <button type="submit" class="btn primary" [disabled]="busy() || !title().trim()">{{ i18n.t('post.submit') }}</button>
        </footer>
      </form>
    </dialog>
  `,
  styles: `
    .wrap { padding: 24px 28px; overflow-y: auto; max-height: inherit; }
    .top { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }
    h2 { margin: 0; font-size: 20px; font-weight: 600; }
    .similar { margin: -4px 0 16px; padding: 10px 14px; border-radius: 12px; background: var(--pb-column); }
    .similar p { margin: 0 0 6px; font-size: 13px; }
    .similar ul { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 6px; }
    .similar li { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; font-size: 14px; }
    .similar a { font-weight: 500; }
    .votes { color: var(--pb-muted); font-size: 12px; }
    .cols { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
    @media (max-width: 600px) { .cols { grid-template-columns: 1fr; } .wrap { padding: 20px 16px; } }
    .actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 8px; }
  `,
})
export class NewPostDialog {
  readonly closed = output<void>();

  protected readonly i18n = inject(I18n);
  protected readonly store = inject(BoardStore);
  protected readonly me = inject(MeStore);
  private readonly api = inject(Api);
  private readonly router = inject(Router);
  private readonly toast = inject(Toast);
  private readonly events = inject(BoardEvents);
  private readonly dialog = viewChild.required<ElementRef<HTMLDialogElement>>('dialog');
  private readonly titleInput = viewChild.required<ElementRef<HTMLInputElement>>('titleInput');

  protected readonly title = signal('');
  protected readonly body = signal('');
  protected readonly tagId = signal<number | null>(null);
  protected readonly images = signal<Image[]>([]);
  protected readonly name = linkedSignal(() => this.me.me().guest?.name ?? '');
  protected readonly busy = signal(false);
  protected readonly errors = signal<Record<string, string>>({});
  protected readonly similar = signal<Similar['posts']>([]);
  private readonly titles = new Subject<string>();
  protected readonly url = postUrl;

  constructor() {
    afterNextRender(() => {
      this.dialog().nativeElement.showModal();
      this.titleInput().nativeElement.focus();
    });
    this.titles
      .pipe(
        debounceTime(300),
        distinctUntilChanged(),
        switchMap((t) => (t.trim().length < 3 ? of({ posts: [] }) : this.api.similar(t).pipe(catchError(() => of({ posts: [] }))))),
        takeUntilDestroyed(),
      )
      .subscribe((r) => this.similar.set(r.posts));
  }

  protected setTitle(v: string): void {
    this.title.set(v);
    this.titles.next(v);
  }

  protected reason(r: string): string {
    const key = `validation.${r}`;
    return this.i18n.has(key) ? this.i18n.t(key as TKey) : r;
  }

  protected async submit(): Promise<void> {
    this.busy.set(true);
    this.errors.set({});
    try {
      const m = this.me.me();
      const wanted = this.name().trim();
      if (!m.admin && wanted !== (m.guest?.name ?? '')) this.me.set(await firstValueFrom(this.api.setName(wanted)));
      const p = await firstValueFrom(
        this.api.createPost({ title: this.title(), body: this.body(), tagId: this.tagId(), imageIds: this.images().map((i) => i.id) }),
      );
      await this.me.refresh();
      this.events.changed();
      if (p.pending) this.toast.show(this.i18n.t('post.pendingNotice'));
      this.closed.emit();
      this.router.navigate([postUrl(p)]);
    } catch (e) {
      if (e instanceof ApiError && e.code === 'validation') this.errors.set(e.fields);
      else this.toast.error(e);
    } finally {
      this.busy.set(false);
    }
  }
}

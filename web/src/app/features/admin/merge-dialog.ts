import { ChangeDetectionStrategy, Component, ElementRef, afterNextRender, inject, input, output, signal, viewChild } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Router } from '@angular/router';
import { catchError, debounceTime, firstValueFrom, of, Subject, switchMap } from 'rxjs';
import { Api } from '../../core/api';
import type { PostCard } from '../../core/api.types';
import { BoardEvents } from '../../core/board-events';
import { I18n } from '../../core/i18n/i18n';
import { postUrl } from '../../core/route-utils';
import { Toast } from '../../core/toast';

@Component({
  selector: 'pb-merge-dialog',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <dialog #dialog class="pb-dialog narrow" [attr.aria-label]="i18n.t('admin.mergeTitle')" (cancel)="$event.preventDefault(); close()">
      <div class="wrap">
        <h2>{{ i18n.t('admin.mergeTitle') }}</h2>
        <p class="muted">{{ i18n.t('admin.mergeHelp') }}</p>
        <input class="input" type="search" [placeholder]="i18n.t('admin.searchPosts')" [attr.aria-label]="i18n.t('admin.searchPosts')"
          (input)="q.next($any($event.target).value)" />
        <ul class="results">
          @for (p of results(); track p.id) {
            <li><span>{{ p.title }}</span><button type="button" class="btn" (click)="merge(p)">{{ i18n.t('admin.mergeInto') }}</button></li>
          }
        </ul>
        <div class="actions-end"><button type="button" class="btn ghost" (click)="close()">{{ i18n.t('common.cancel') }}</button></div>
      </div>
    </dialog>
  `,
  styles: `.wrap { padding: 24px; } h2 { margin: 0 0 6px; font-size: 20px; }
    .results { list-style: none; margin: 12px 0; padding: 0; display: flex; flex-direction: column; gap: 6px; max-height: 320px; overflow-y: auto; }
    .results li { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 8px 10px; border-radius: 10px; background: var(--pb-column); }`,
})
export class MergeDialog {
  readonly postId = input.required<number>();
  readonly closed = output<void>();
  protected readonly i18n = inject(I18n);
  private readonly api = inject(Api);
  private readonly router = inject(Router);
  private readonly toast = inject(Toast);
  private readonly events = inject(BoardEvents);
  private readonly dialog = viewChild.required<ElementRef<HTMLDialogElement>>('dialog');
  protected readonly q = new Subject<string>();
  protected readonly results = signal<PostCard[]>([]);

  constructor() {
    afterNextRender(() => this.dialog().nativeElement.showModal());
    this.q
      .pipe(
        debounceTime(250),
        switchMap((q) => (q.trim() ? this.api.listPosts({ q }).pipe(catchError(() => of({ posts: [], nextOffset: null }))) : of({ posts: [], nextOffset: null }))),
        takeUntilDestroyed(),
      )
      .subscribe((page) => this.results.set(page.posts.filter((p) => p.id !== this.postId())));
  }

  protected close(): void {
    this.dialog().nativeElement.close();
    this.closed.emit();
  }

  protected async merge(target: PostCard): Promise<void> {
    try {
      const p = await firstValueFrom(this.api.merge(this.postId(), target.id));
      this.dialog().nativeElement.close();
      this.events.changed();
      this.router.navigate([postUrl(p)], { replaceUrl: true });
    } catch (e) {
      this.toast.error(e);
    }
  }
}

import { ChangeDetectionStrategy, Component, ElementRef, afterNextRender, inject, input, linkedSignal, output, signal, viewChild } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { catchError, debounceTime, firstValueFrom, of, Subject, switchMap } from 'rxjs';
import { Api } from '../../core/api';
import type { PostCard, UpdateItem } from '../../core/api.types';
import { I18n } from '../../core/i18n/i18n';
import { Toast } from '../../core/toast';
import { Icon } from '../../shared/icon';
import { MarkdownEditor } from '../../shared/markdown-editor/markdown-editor';
import { fieldMessage, fieldsOf } from './field-errors';

type Linked = { id: number; title: string };

@Component({
  selector: 'pb-update-editor-dialog',
  imports: [Icon, MarkdownEditor],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <dialog #dialog class="pb-dialog" [attr.aria-label]="i18n.t('updates.new')" (cancel)="$event.preventDefault(); close()">
      <form class="wrap" (submit)="$event.preventDefault(); save()">
        <h2>{{ update() ? update()!.title : i18n.t('updates.new') }}</h2>
        <label class="field"><span>{{ i18n.t('update.titleLabel') }}</span>
          <input class="input" maxlength="120" [value]="title()" (input)="title.set($any($event.target).value)" /><span class="field-error">{{ msg('title') }}</span></label>
        <div class="field"><span>{{ i18n.t('update.bodyLabel') }}</span>
          <pb-markdown-editor [(value)]="body" [rows]="5" [label]="i18n.t('update.bodyLabel')" /><span class="field-error">{{ msg('body') }}</span></div>
        <div class="field"><span>{{ i18n.t('update.postsLabel') }}</span>
          <div class="chips">
            @for (p of linked(); track p.id) {
              <span class="chip">{{ p.title }}<button type="button" class="x" [attr.aria-label]="i18n.t('common.delete')" (click)="unlink(p)"><pb-icon name="x" /></button></span>
            }
          </div>
          @if (linked().length < 10) {
            <input class="input" type="search" [placeholder]="i18n.t('admin.searchPosts')" (input)="q.next($any($event.target).value)" />
            <ul class="results">
              @for (r of results(); track r.id) {<li><button type="button" class="btn ghost" (click)="link(r)"><pb-icon name="plus" />{{ r.title }}</button></li>}
            </ul>
          }
          <span class="field-error">{{ msg('postIds') }}</span>
        </div>
        <div class="actions-end">
          <button type="button" class="btn ghost" (click)="close()">{{ i18n.t('common.cancel') }}</button>
          <button type="submit" class="btn primary" [disabled]="busy()">{{ i18n.t('common.save') }}</button>
        </div>
      </form>
    </dialog>
  `,
  styles: `.wrap { padding: 24px; } h2 { margin: 0 0 16px; font-size: 20px; }
    .chips { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 8px; }
    .x { display: inline-flex; padding: 0; border: 0; background: none; color: inherit; cursor: pointer; } .x pb-icon { width: 14px; height: 14px; }
    .results { list-style: none; margin: 6px 0 0; padding: 0; } .results .btn { justify-content: flex-start; text-align: left; }`,
})
export class UpdateEditorDialog {
  readonly update = input<UpdateItem | null>(null);
  readonly closed = output<boolean>();
  protected readonly i18n = inject(I18n);
  private readonly api = inject(Api);
  private readonly toast = inject(Toast);
  private readonly dialog = viewChild.required<ElementRef<HTMLDialogElement>>('dialog');
  protected readonly title = linkedSignal(() => this.update()?.title ?? '');
  protected readonly body = linkedSignal(() => this.update()?.bodyMd ?? '');
  protected readonly linked = linkedSignal<Linked[]>(() => (this.update()?.posts ?? []).map((p) => ({ id: p.id, title: p.title })));
  protected readonly results = signal<PostCard[]>([]);
  protected readonly busy = signal(false);
  private readonly errors = signal<Record<string, string>>({});
  protected readonly q = new Subject<string>();

  constructor() {
    afterNextRender(() => this.dialog().nativeElement.showModal());
    this.q
      .pipe(
        debounceTime(250),
        switchMap((q) => (q.trim() ? this.api.listPosts({ q }).pipe(catchError(() => of({ posts: [], nextOffset: null }))) : of({ posts: [], nextOffset: null }))),
        takeUntilDestroyed(),
      )
      .subscribe((p) => this.results.set(p.posts.filter((r) => !this.linked().some((l) => l.id === r.id))));
  }

  protected msg(f: string): string {
    return fieldMessage(this.i18n, this.errors()[f]);
  }

  protected link(p: PostCard): void {
    this.linked.update((l) => [...l, { id: p.id, title: p.title }]);
    this.results.update((r) => r.filter((x) => x.id !== p.id));
  }

  protected unlink(p: Linked): void {
    this.linked.update((l) => l.filter((x) => x.id !== p.id));
  }

  protected close(saved = false): void {
    this.dialog().nativeElement.close();
    this.closed.emit(saved);
  }

  protected async save(): Promise<void> {
    this.busy.set(true);
    this.errors.set({});
    const body = { title: this.title(), body: this.body(), postIds: this.linked().map((l) => l.id) };
    try {
      const u = this.update();
      await firstValueFrom(u ? this.api.editUpdate(u.id, body) : this.api.createUpdate(body));
      this.close(true);
    } catch (e) {
      const f = fieldsOf(e);
      if (Object.keys(f).length) this.errors.set(f);
      else this.toast.error(e);
    } finally {
      this.busy.set(false);
    }
  }
}

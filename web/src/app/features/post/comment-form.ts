import { ChangeDetectionStrategy, Component, inject, input, linkedSignal, output, signal, viewChild } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { Api } from '../../core/api';
import type { Comment } from '../../core/api.types';
import { I18n } from '../../core/i18n/i18n';
import { MeStore } from '../../core/me.store';
import { Toast } from '../../core/toast';
import { Avatar } from '../../shared/avatar';
import { MarkdownEditor } from '../../shared/markdown-editor/markdown-editor';

@Component({
  selector: 'pb-comment-form',
  imports: [Avatar, MarkdownEditor],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <form class="form" (submit)="$event.preventDefault(); submit()">
      <pb-markdown-editor #editor [(value)]="body" [placeholder]="i18n.t('comment.placeholder')" [maxLength]="5000" [rows]="parentId() ? 2 : 3" />
      <div class="foot">
        @if (me.me().admin; as a) {
          <pb-avatar [seed]="'admin-' + a.id" [size]="28" /><span class="who">{{ a.displayName }} <span class="team-badge">{{ i18n.t('author.team') }}</span></span>
        } @else {
          <pb-avatar [seed]="me.me().guest?.id ?? 'new'" [size]="28" />
          <input class="name" type="text" maxlength="40" [placeholder]="i18n.t('comment.namePlaceholder')"
            [attr.aria-label]="i18n.t('comment.namePlaceholder')" [value]="name()" (input)="name.set($any($event.target).value)" />
        }
        <span class="spacer"></span>
        <button type="button" class="btn ghost" (click)="cancel()">{{ i18n.t('common.cancel') }}</button>
        <button type="submit" class="btn primary" [disabled]="!body().trim() || busy()">{{ i18n.t('comment.submit') }}</button>
      </div>
    </form>
  `,
  styles: `
    .foot { display: flex; align-items: center; gap: 10px; margin-top: 10px; flex-wrap: wrap; }
    .name { flex: 1 1 160px; min-width: 0; padding: 6px 8px; border: 0; border-bottom: 1px solid transparent; background: transparent; outline: none; }
    .name:focus { border-bottom-color: var(--pb-border); }
    .who { font-size: 14px; font-weight: 500; }
    .spacer { flex: 1; }
  `,
})
export class CommentForm {
  readonly postId = input.required<number>();
  readonly parentId = input<number | null>(null);
  readonly posted = output<Comment>();
  readonly cancelled = output<void>();

  protected readonly i18n = inject(I18n);
  protected readonly me = inject(MeStore);
  private readonly api = inject(Api);
  private readonly toast = inject(Toast);
  private readonly editor = viewChild.required<MarkdownEditor>('editor');
  protected readonly body = signal('');
  protected readonly name = linkedSignal(() => this.me.me().guest?.name ?? '');
  protected readonly busy = signal(false);

  focus(): void {
    this.editor().focus();
  }

  protected cancel(): void {
    this.body.set('');
    this.cancelled.emit();
  }

  protected async submit(): Promise<void> {
    if (!this.body().trim() || this.busy()) return;
    this.busy.set(true);
    try {
      const m = this.me.me();
      const wanted = this.name().trim();
      if (!m.admin && wanted !== (m.guest?.name ?? '')) this.me.set(await firstValueFrom(this.api.setName(wanted)));
      const c = await firstValueFrom(this.api.comment(this.postId(), this.body(), this.parentId()));
      this.body.set('');
      if (!this.me.me().guest && !this.me.me().admin) await this.me.refresh();
      this.posted.emit(c);
    } catch (e) {
      this.toast.error(e);
    } finally {
      this.busy.set(false);
    }
  }
}

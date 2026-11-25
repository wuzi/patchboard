import { NgTemplateOutlet } from '@angular/common';
import { ChangeDetectionStrategy, Component, inject, input, output, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { Api } from '../../core/api';
import type { Comment } from '../../core/api.types';
import { I18n } from '../../core/i18n/i18n';
import { MeStore } from '../../core/me.store';
import { Toast } from '../../core/toast';
import { Avatar } from '../../shared/avatar';
import { Icon } from '../../shared/icon';
import { MarkdownEditor } from '../../shared/markdown-editor/markdown-editor';
import { BanForm } from '../admin/ban-form';
import { CommentForm } from './comment-form';

@Component({
  selector: 'pb-comment-thread',
  imports: [NgTemplateOutlet, Avatar, Icon, CommentForm, MarkdownEditor, BanForm],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <ol class="thread">
      @for (c of comments(); track c.id) {
        <li>
          <ng-container *ngTemplateOutlet="one; context: { $implicit: c, top: true }" />
          @if (c.replies.length || replyTo() === c.id) {
            <ol class="replies" data-testid="replies">
              @for (r of c.replies; track r.id) {<li><ng-container *ngTemplateOutlet="one; context: { $implicit: r, top: false }" /></li>}
            </ol>
          }
          @if (replyTo() === c.id) {
            <div class="reply-form" data-testid="reply-form">
              <pb-comment-form [postId]="postId()" [parentId]="c.id" (posted)="replyTo.set(null); changed.emit()" (cancelled)="replyTo.set(null)" />
            </div>
          }
        </li>
      }
    </ol>

    <ng-template #one let-c let-top="top">
      @if (c.deleted) {
        <p class="deleted">{{ i18n.t('comment.deleted') }}</p>
      } @else {
        <article class="comment">
          <pb-avatar [seed]="c.author.id" [size]="32" />
          <div class="main">
            <p class="meta"><strong>{{ c.author.name ?? i18n.t('author.anonymous') }}</strong>
              @if (c.author.kind === 'admin') {<span class="team-badge">{{ i18n.t('author.team') }}</span>}
              <span class="muted">{{ i18n.date(c.createdAt) }}</span></p>
            @if (editing() === c.id) {
              <pb-markdown-editor [(value)]="draft" [maxLength]="5000" [rows]="3" />
              <div class="row"><button type="button" class="btn ghost" (click)="editing.set(null)">{{ i18n.t('common.cancel') }}</button>
                <button type="button" class="btn primary" [disabled]="!draft().trim()" (click)="save(c)">{{ i18n.t('common.save') }}</button></div>
            } @else {
              <div class="md body" [innerHTML]="c.bodyHtml"></div>
            }
            <div class="actions">
              <button type="button" class="act like" [class.on]="c.liked" [attr.aria-label]="i18n.t(c.liked ? 'comment.unlike' : 'comment.like')"
                [attr.aria-pressed]="c.liked" (click)="toggleLike(c)"><pb-icon name="heart" />@if (c.likeCount) {{{ c.likeCount }}}</button>
              @if (top && (!locked() || me.isAdmin())) {
                <button type="button" class="act" (click)="replyTo.set(c.id)">{{ i18n.t('comment.reply') }}</button>
              }
              @if (me.isAdmin() && c.author.kind === 'guest') {
                <button type="button" class="act" (click)="banning.set(banning() === c.id ? null : c.id)">{{ i18n.t('admin.banAuthor') }}</button>
              }
              @if (c.canEdit && editing() !== c.id) {
                <button type="button" class="act" (click)="edit(c)">{{ i18n.t('common.edit') }}</button>
                <button type="button" class="act" (click)="remove(c)">{{ i18n.t('common.delete') }}</button>
              }
            </div>
            @if (banning() === c.id) {<pb-ban-form [guestId]="c.author.id" (done)="banning.set(null)" (cancelled)="banning.set(null)" />}
          </div>
        </article>
      }
    </ng-template>
  `,
  styles: `
    ol { list-style: none; margin: 0; padding: 0; }
    .thread > li + li { margin-top: 18px; }
    .replies { margin: 12px 0 0 42px; display: flex; flex-direction: column; gap: 12px; }
    .reply-form { margin: 12px 0 0 42px; }
    .comment { display: flex; gap: 10px; }
    .main { flex: 1; min-width: 0; }
    .meta { display: flex; align-items: center; gap: 8px; margin: 2px 0 4px; font-size: 14px; flex-wrap: wrap; }
    .meta strong { font-weight: 600; }
    .body { font-size: 15px; }
    .actions { display: flex; gap: 4px; margin-top: 4px; margin-left: -8px; }
    .act { display: inline-flex; align-items: center; gap: 4px; padding: 4px 8px; border: 0; border-radius: 8px; background: transparent;
      color: var(--pb-muted); font-size: 13px; font-weight: 500; cursor: pointer; }
    .act:hover { background: var(--pb-hover); color: var(--pb-text); }
    .act pb-icon { width: 16px; height: 16px; }
    .like.on { color: #e11d48; }
    .like.on pb-icon { fill: currentColor; }
    .deleted { margin: 0; color: var(--pb-muted); font-style: italic; font-size: 14px; }
    .row { display: flex; justify-content: flex-end; gap: 8px; margin-top: 8px; }
  `,
})
export class CommentThread {
  readonly postId = input.required<number>();
  readonly comments = input.required<Comment[]>();
  readonly locked = input(false);
  readonly changed = output<void>();

  protected readonly i18n = inject(I18n);
  protected readonly me = inject(MeStore);
  private readonly api = inject(Api);
  private readonly toast = inject(Toast);
  protected readonly replyTo = signal<number | null>(null);
  protected readonly editing = signal<number | null>(null);
  protected readonly draft = signal('');
  protected readonly banning = signal<number | null>(null);

  protected async toggleLike(c: Comment): Promise<void> {
    try {
      const s = await firstValueFrom(c.liked ? this.api.unlike(c.id) : this.api.like(c.id));
      c.liked = s.liked;
      c.likeCount = s.likeCount;
      this.changed.emit();
      if (!this.me.me().guest) await this.me.refresh();
    } catch (e) {
      this.toast.error(e);
    }
  }

  protected edit(c: Comment): void {
    this.draft.set(c.bodyMd ?? '');
    this.editing.set(c.id);
  }

  protected async save(c: Comment): Promise<void> {
    try {
      await firstValueFrom(this.api.editComment(c.id, this.draft()));
      this.editing.set(null);
      this.changed.emit();
    } catch (e) {
      this.toast.error(e);
    }
  }

  protected async remove(c: Comment): Promise<void> {
    if (!confirm(this.i18n.t('comment.confirmDelete'))) return;
    try {
      await firstValueFrom(this.api.deleteComment(c.id));
      this.changed.emit();
    } catch (e) {
      this.toast.error(e);
    }
  }
}

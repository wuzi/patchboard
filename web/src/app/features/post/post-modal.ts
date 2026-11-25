import {
  ChangeDetectionStrategy, Component, ElementRef, afterNextRender, computed, effect, inject, input, signal, untracked, viewChild,
} from '@angular/core';
import { Title } from '@angular/platform-browser';
import { Router } from '@angular/router';
import { firstValueFrom } from 'rxjs';
import { Api } from '../../core/api';
import type { PostDetail } from '../../core/api.types';
import { BoardEvents } from '../../core/board-events';
import { BoardStore } from '../../core/board.store';
import { I18n } from '../../core/i18n/i18n';
import { MeStore } from '../../core/me.store';
import { NavState } from '../../core/nav-state';
import { parseIdSlug, postUrl } from '../../core/route-utils';
import { Toast } from '../../core/toast';
import { Avatar } from '../../shared/avatar';
import { Icon } from '../../shared/icon';
import { MarkdownEditor } from '../../shared/markdown-editor/markdown-editor';
import { StatusChip } from '../../shared/status-chip';
import { VoteButton } from '../../shared/vote-button';
import { CommentForm } from './comment-form';
import { CommentThread } from './comment-thread';
import { PostAdminBar } from './post-admin-bar';

@Component({
  selector: 'pb-post-modal',
  imports: [Avatar, Icon, MarkdownEditor, StatusChip, VoteButton, CommentForm, CommentThread, PostAdminBar],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './post-modal.html',
  styleUrl: './post-modal.scss',
})
export class PostModal {
  readonly idSlug = input.required<string>();

  protected readonly i18n = inject(I18n);
  protected readonly me = inject(MeStore);
  protected readonly store = inject(BoardStore);
  private readonly api = inject(Api);
  private readonly router = inject(Router);
  private readonly nav = inject(NavState);
  private readonly toast = inject(Toast);
  private readonly events = inject(BoardEvents);
  private readonly title = inject(Title);
  private readonly dialog = viewChild.required<ElementRef<HTMLDialogElement>>('dialog');

  protected readonly post = signal<PostDetail | null>(null);
  protected readonly tag = computed(() => this.store.tag(this.post()?.tagId));
  protected readonly editing = signal(false);
  protected readonly draftTitle = signal('');
  protected readonly draftBody = signal('');
  private readonly ref = computed(() => parseIdSlug(this.idSlug()));

  constructor() {
    afterNextRender(() => this.dialog().nativeElement.showModal());
    effect(() => {
      const ref = this.ref();
      untracked(() => (ref ? this.load(ref.id) : this.notFound()));
    });
  }

  protected async load(id: number): Promise<void> {
    try {
      const res = await firstValueFrom(this.api.post(id));
      if (res.mergedInto) {
        this.router.navigate([postUrl(res.mergedInto)], { replaceUrl: true, queryParamsHandling: 'preserve' });
        return;
      }
      const p = res.post!;
      this.post.set(p);
      this.title.setTitle(p.title);
      if (this.ref()?.slug !== p.slug) this.router.navigate([postUrl(p)], { replaceUrl: true, queryParamsHandling: 'preserve' });
    } catch (e) {
      this.toast.error(e);
      this.close();
    }
  }

  protected reload(): void {
    const p = this.post();
    if (p) this.load(p.id);
  }

  private notFound(): void {
    this.toast.show(this.i18n.t('error.not_found'), 'error');
    this.close();
  }

  close(): void {
    this.router.navigateByUrl(this.nav.lastUrl());
  }

  protected onCancel(e: Event): void {
    e.preventDefault();
    this.close();
  }

  protected onBackdrop(e: MouseEvent): void {
    if (e.target === this.dialog().nativeElement) this.close();
  }

  protected authorName(p: PostDetail): string {
    return p.author.name ?? this.i18n.t('author.anonymous');
  }

  protected async share(p: PostDetail): Promise<void> {
    try {
      await navigator.clipboard.writeText(location.origin + postUrl(p));
      this.toast.show(this.i18n.t('post.linkCopied'));
    } catch {
      this.toast.show(location.origin + postUrl(p));
    }
  }

  protected startEdit(p: PostDetail): void {
    this.draftTitle.set(p.title);
    this.draftBody.set(p.bodyMd);
    this.editing.set(true);
  }

  protected async saveEdit(p: PostDetail): Promise<void> {
    try {
      const updated = await firstValueFrom(this.api.updatePost(p.id, { title: this.draftTitle(), body: this.draftBody() }));
      this.post.set(updated);
      this.editing.set(false);
      this.events.changed();
      if (updated.slug !== p.slug) this.router.navigate([postUrl(updated)], { replaceUrl: true, queryParamsHandling: 'preserve' });
    } catch (e) {
      this.toast.error(e);
    }
  }

  protected async remove(p: PostDetail): Promise<void> {
    if (!confirm(this.i18n.t('post.confirmDelete'))) return;
    try {
      await firstValueFrom(this.api.deletePost(p.id));
      this.events.changed();
      this.close();
    } catch (e) {
      this.toast.error(e);
    }
  }

  protected async deleteImage(id: string): Promise<void> {
    if (!confirm(this.i18n.t('admin.deleteImage') + '?')) return;
    try {
      await firstValueFrom(this.api.deleteUpload(id));
      this.reload();
    } catch (e) {
      this.toast.error(e);
    }
  }

  protected onVote(): void {
    this.reload();
  }

  protected onCommented(): void {
    this.reload();
    this.events.changed();
  }
}

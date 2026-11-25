import { ChangeDetectionStrategy, Component, inject, input, output, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { Api } from '../../core/api';
import type { PostDetail } from '../../core/api.types';
import { BoardEvents } from '../../core/board-events';
import { BoardStore } from '../../core/board.store';
import { I18n } from '../../core/i18n/i18n';
import { Toast } from '../../core/toast';
import { Icon } from '../../shared/icon';
import { MergeDialog } from '../admin/merge-dialog';
import { StatusChangeDialog } from '../admin/status-change-dialog';
import { VoteAuditDialog } from '../admin/vote-audit-dialog';

/** Team-only controls inside the post modal. */
@Component({
  selector: 'pb-post-admin-bar',
  imports: [Icon, MergeDialog, StatusChangeDialog, VoteAuditDialog],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @let p = post();
    <div class="bar">
      <label class="status"><span>{{ i18n.t('status.label') }}</span>
        <select class="input" (change)="pick(+$any($event.target).value, $event)">
          @for (s of store.statuses(); track s.id) {<option [value]="s.id" [selected]="s.id === p.statusId">{{ i18n.name(s.name) }}</option>}
        </select></label>
      <button type="button" class="btn ghost" (click)="changeTo.set(p.statusId)"><pb-icon name="comment" />{{ i18n.t('admin.respond') }}</button>
      @if (p.pending) {<button type="button" class="btn ghost" (click)="act(api.approve(p.id))"><pb-icon name="check" />{{ i18n.t('admin.approve') }}</button>}
      <button type="button" class="btn ghost" (click)="act(api.pin(p.id, !p.pinned))"><pb-icon name="pin" />{{ i18n.t(p.pinned ? 'admin.unpin' : 'admin.pin') }}</button>
      <button type="button" class="btn ghost" (click)="act(api.lock(p.id, !p.commentsLocked))"><pb-icon [name]="p.commentsLocked ? 'unlock' : 'lock'" />{{ i18n.t(p.commentsLocked ? 'admin.unlock' : 'admin.lock') }}</button>
      <button type="button" class="btn ghost" (click)="audit.set(true)"><pb-icon name="up" />{{ i18n.t('admin.votes') }}</button>
      <button type="button" class="btn ghost" (click)="merging.set(true)"><pb-icon name="merge" />{{ i18n.t('admin.merge') }}</button>
      @if (p.deleted) {
        <button type="button" class="btn ghost" (click)="restore()"><pb-icon name="restore" />{{ i18n.t('common.restore') }}</button>
      }
    </div>
    @if (changeTo() !== null) {
      <pb-status-change-dialog [post]="p" [toStatusId]="changeTo()" (done)="statusChanged()" (cancelled)="changeTo.set(null)" />
    }
    @if (merging()) {<pb-merge-dialog [postId]="p.id" (closed)="merging.set(false)" />}
    @if (audit()) {<pb-vote-audit-dialog [postId]="p.id" (closed)="audit.set(false); changed.emit()" />}
  `,
  styles: `
    .bar { display: flex; align-items: center; gap: 4px; flex-wrap: wrap; padding: 10px 12px; margin-bottom: 16px; border-radius: 14px;
      background: var(--pb-column); }
    .status { display: inline-flex; align-items: center; gap: 8px; margin-right: 6px; font-size: 13px; color: var(--pb-muted); }
    .status .input { width: auto; padding: 6px 10px; }
    .btn { min-height: 32px; padding: 4px 10px; font-size: 13px; }
  `,
})
export class PostAdminBar {
  readonly post = input.required<PostDetail>();
  readonly changed = output<void>();
  protected readonly i18n = inject(I18n);
  protected readonly store = inject(BoardStore);
  protected readonly api = inject(Api);
  private readonly toast = inject(Toast);
  private readonly events = inject(BoardEvents);
  protected readonly changeTo = signal<number | null>(null);
  protected readonly merging = signal(false);
  protected readonly audit = signal(false);

  protected pick(statusId: number, e: Event): void {
    (e.target as HTMLSelectElement).value = String(this.post().statusId); // the dialog decides
    this.changeTo.set(statusId);
  }

  protected statusChanged(): void {
    this.changeTo.set(null);
    this.events.changed();
    this.changed.emit();
  }

  protected async act(op: ReturnType<Api['pin']>): Promise<void> {
    try {
      await firstValueFrom(op);
      this.events.changed();
      this.changed.emit();
    } catch (e) {
      this.toast.error(e);
    }
  }

  protected async restore(): Promise<void> {
    try {
      await firstValueFrom(this.api.restorePost(this.post().id));
      this.events.changed();
      this.changed.emit();
    } catch (e) {
      this.toast.error(e);
    }
  }
}

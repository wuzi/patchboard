import { ChangeDetectionStrategy, Component, ElementRef, afterNextRender, computed, inject, input, linkedSignal, output, signal, viewChild } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { Api } from '../../core/api';
import type { ChangeStatus, PostDetail } from '../../core/api.types';
import { BoardStore } from '../../core/board.store';
import { I18n } from '../../core/i18n/i18n';
import { Toast } from '../../core/toast';
import { MarkdownEditor } from '../../shared/markdown-editor/markdown-editor';

/** Change a post's status, optionally with a team response and a published Update. */
@Component({
  selector: 'pb-status-change-dialog',
  imports: [MarkdownEditor],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <dialog #dialog class="pb-dialog narrow" [attr.aria-label]="i18n.t('status.dialogTitle')" (cancel)="$event.preventDefault(); cancel()">
      <form class="wrap" (submit)="$event.preventDefault(); confirm()">
        <h2>{{ i18n.t('status.dialogTitle') }}</h2>
        <p class="muted post-title">{{ post().title }}</p>
        <label class="field"><span>{{ i18n.t('status.label') }}</span>
          <select class="input" (change)="statusId.set(+$any($event.target).value)">
            @for (s of store.statuses(); track s.id) {<option [value]="s.id" [selected]="s.id === statusId()">{{ i18n.name(s.name) }}</option>}
          </select></label>
        <div class="field"><span>{{ i18n.t('post.teamResponse') }}</span>
          <pb-markdown-editor [value]="response()" (valueChange)="setResponse($event)" [placeholder]="i18n.t('status.responsePlaceholder')" [maxLength]="5000" [rows]="3" />
        </div>
        <label class="check"><input type="checkbox" [checked]="publish()" (change)="publish.set($any($event.target).checked)" />{{ i18n.t('status.publishUpdate') }}</label>
        @if (publish()) {
          <label class="field"><span>{{ i18n.t('status.updateTitleLabel') }}</span>
            <input class="input" maxlength="120" [value]="updateTitle()" (input)="updateTitle.set($any($event.target).value)" /></label>
          <label class="field"><span>{{ i18n.t('status.updateBodyLabel') }}</span>
            <textarea class="input" rows="3" [value]="updateBody()" (input)="updateBody.set($any($event.target).value)"></textarea></label>
        }
        <div class="actions-end">
          <button type="button" class="btn ghost" (click)="cancel()">{{ i18n.t('common.cancel') }}</button>
          <button type="submit" class="btn primary" [disabled]="busy() || (publish() && (!updateTitle().trim() || !updateBody().trim()))">{{ i18n.t('common.confirm') }}</button>
        </div>
      </form>
    </dialog>
  `,
  styles: `.wrap { padding: 24px; } h2 { margin: 0; font-size: 20px; font-weight: 600; } .post-title { margin: 4px 0 16px; }`,
})
export class StatusChangeDialog {
  readonly post = input.required<{ id: number; title: string; statusId: number; response?: PostDetail['response'] }>();
  readonly toStatusId = input<number | null>(null);
  readonly done = output<PostDetail>();
  readonly cancelled = output<void>();

  protected readonly i18n = inject(I18n);
  protected readonly store = inject(BoardStore);
  private readonly api = inject(Api);
  private readonly toast = inject(Toast);
  private readonly dialog = viewChild.required<ElementRef<HTMLDialogElement>>('dialog');

  protected readonly statusId = linkedSignal(() => this.toStatusId() ?? this.post().statusId);
  protected readonly response = linkedSignal(() => this.post().response?.md ?? '');
  private readonly responseTouched = signal(false);
  protected readonly publish = signal(false);
  private readonly statusName = computed(() => this.i18n.name(this.store.status(this.statusId())?.name));
  protected readonly updateTitle = linkedSignal(() => this.i18n.t('status.updateTitle', { post: this.post().title, status: this.statusName() }));
  protected readonly updateBody = linkedSignal(() => this.response());
  protected readonly busy = signal(false);

  constructor() {
    afterNextRender(() => this.dialog().nativeElement.showModal());
  }

  protected setResponse(v: string): void {
    this.response.set(v);
    this.responseTouched.set(true);
  }

  protected cancel(): void {
    this.dialog().nativeElement.close();
    this.cancelled.emit();
  }

  protected async confirm(): Promise<void> {
    const body: ChangeStatus = { statusId: this.statusId() };
    if (this.responseTouched()) body.response = this.response();
    if (this.publish()) body.update = { title: this.updateTitle(), body: this.updateBody() };
    this.busy.set(true);
    try {
      const p = await firstValueFrom(this.api.changeStatus(this.post().id, body));
      this.dialog().nativeElement.close();
      this.done.emit(p);
    } catch (e) {
      this.toast.error(e);
    } finally {
      this.busy.set(false);
    }
  }
}

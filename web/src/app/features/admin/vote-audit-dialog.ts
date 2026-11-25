import { ChangeDetectionStrategy, Component, ElementRef, afterNextRender, inject, input, output, signal, viewChild } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { Api } from '../../core/api';
import type { VoteAudit } from '../../core/api.types';
import { I18n, type TKey } from '../../core/i18n/i18n';
import { Toast } from '../../core/toast';
import { Avatar } from '../../shared/avatar';
import { BanForm } from './ban-form';

type Voter = VoteAudit['voters'][number];

@Component({
  selector: 'pb-vote-audit-dialog',
  imports: [Avatar, BanForm],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <dialog #dialog class="pb-dialog" [attr.aria-label]="i18n.t('admin.voteAuditTitle')" (cancel)="$event.preventDefault(); close()">
      <div class="wrap">
        <h2>{{ i18n.t('admin.voteAuditTitle') }}</h2>
        @if (voters().length === 0) {<p class="empty">{{ i18n.t('common.empty') }}</p>}
        <ul class="voters">
          @for (v of voters(); track v.guest.id) {
            <li>
              <div class="who"><pb-avatar [seed]="v.guest.id" [size]="26" />
                <strong>{{ v.guest.name ?? i18n.t('author.anonymous') }}</strong>
                <span class="muted mono">{{ v.ipHash }}</span><span class="muted">{{ i18n.date(v.votedAt) }}</span>
                @for (f of v.flags; track f) {<span class="chip flag">{{ i18n.t(flagKey(f)) }}</span>}</div>
              <div class="acts">
                <button type="button" class="btn ghost" (click)="remove(v)">{{ i18n.t('admin.removeVote') }}</button>
                <button type="button" class="btn ghost danger" (click)="banning.set(v.guest.id)">{{ i18n.t('admin.ban') }}</button>
              </div>
              @if (banning() === v.guest.id) {
                <pb-ban-form [guestId]="v.guest.id" (done)="banning.set(null)" (cancelled)="banning.set(null)" />
              }
            </li>
          }
        </ul>
        <div class="actions-end"><button type="button" class="btn" (click)="close()">{{ i18n.t('common.close') }}</button></div>
      </div>
    </dialog>
  `,
  styles: `.wrap { padding: 24px; } h2 { margin: 0 0 12px; font-size: 20px; }
    .voters { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 8px; max-height: 60vh; overflow-y: auto; }
    .voters li { padding: 10px 12px; border-radius: 12px; background: var(--pb-column); }
    .who { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; font-size: 14px; }
    .mono { font-family: ui-monospace, monospace; font-size: 12px; }
    .flag { background: color-mix(in srgb, #f59e0b 20%, transparent); color: #b45309; }
    .acts { display: flex; gap: 4px; margin-top: 6px; }`,
})
export class VoteAuditDialog {
  readonly postId = input.required<number>();
  readonly closed = output<void>();
  protected readonly i18n = inject(I18n);
  private readonly api = inject(Api);
  private readonly toast = inject(Toast);
  private readonly dialog = viewChild.required<ElementRef<HTMLDialogElement>>('dialog');
  protected readonly voters = signal<Voter[]>([]);
  protected readonly banning = signal<string | null>(null);

  constructor() {
    afterNextRender(() => {
      this.dialog().nativeElement.showModal();
      this.load();
    });
  }

  protected flagKey(f: string): TKey {
    return `admin.flag_${f}` as TKey;
  }

  private async load(): Promise<void> {
    try {
      this.voters.set((await firstValueFrom(this.api.voteAudit(this.postId()))).voters);
    } catch (e) {
      this.toast.error(e);
    }
  }

  protected async remove(v: Voter): Promise<void> {
    try {
      await firstValueFrom(this.api.removeVote(this.postId(), v.guest.id));
      await this.load();
    } catch (e) {
      this.toast.error(e);
    }
  }

  protected close(): void {
    this.dialog().nativeElement.close();
    this.closed.emit();
  }
}

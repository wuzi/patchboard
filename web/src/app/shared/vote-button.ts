import { ChangeDetectionStrategy, Component, inject, input, linkedSignal, output, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { Api } from '../core/api';
import type { VoteState } from '../core/api.types';
import { BoardEvents } from '../core/board-events';
import { I18n } from '../core/i18n/i18n';
import { MeStore } from '../core/me.store';
import { Toast } from '../core/toast';
import { Icon } from './icon';

@Component({
  selector: 'pb-vote-button',
  imports: [Icon],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <button type="button" class="vote" [class.lg]="size() === 'lg'" [class.on]="isVoted()" [disabled]="busy()"
      [attr.aria-pressed]="isVoted()" [attr.aria-label]="i18n.t(isVoted() ? 'post.unvote' : 'post.vote')" (click)="toggle($event)">
      <pb-icon name="up" [weight]="2.4" />{{ total() }}
    </button>
  `,
  styles: `
    :host { position: relative; z-index: 1; display: inline-flex; }
    .vote { display: inline-flex; align-items: center; gap: 2px; height: 32px; min-width: 52px; padding: 0 12px 0 8px;
      border-radius: 10px; border: 1px solid var(--pb-border); background: var(--pb-surface); cursor: pointer;
      font-size: 13px; font-weight: 600; font-variant-numeric: tabular-nums; transition: background .12s; }
    .vote:hover { background: var(--pb-hover); }
    .vote pb-icon { width: 15px; height: 15px; }
    .vote.on { background: var(--pb-strong); border-color: var(--pb-strong); color: var(--pb-on-strong); }
    .vote:disabled { cursor: progress; }
    .vote.lg { flex-direction: column; justify-content: center; gap: 0; width: 64px; height: 64px; padding: 0;
      border-radius: 16px; font-size: 18px; }
    .vote.lg pb-icon { width: 20px; height: 20px; }
  `,
})
export class VoteButton {
  readonly postId = input.required<number>();
  readonly count = input.required<number>();
  readonly voted = input.required<boolean>();
  readonly size = input<'sm' | 'lg'>('sm');
  readonly changed = output<VoteState>();

  protected readonly i18n = inject(I18n);
  private readonly api = inject(Api);
  private readonly me = inject(MeStore);
  private readonly toast = inject(Toast);
  private readonly events = inject(BoardEvents);
  protected readonly total = linkedSignal(() => this.count());
  protected readonly isVoted = linkedSignal(() => this.voted());
  protected readonly busy = signal(false);

  async toggle(e: Event): Promise<void> {
    e.preventDefault();
    e.stopPropagation();
    const was = { voteCount: this.total(), voted: this.isVoted() };
    const up = !was.voted;
    this.isVoted.set(up);
    this.total.set(was.voteCount + (up ? 1 : -1));
    this.busy.set(true);
    try {
      const state = await firstValueFrom(up ? this.api.vote(this.postId()) : this.api.unvote(this.postId()));
      this.total.set(state.voteCount);
      this.isVoted.set(state.voted);
      this.changed.emit(state);
      this.events.votes.next({ postId: this.postId(), state });
      if (!this.me.me().guest) await this.me.refresh();
    } catch (err) {
      this.total.set(was.voteCount);
      this.isVoted.set(was.voted);
      this.toast.error(err);
    } finally {
      this.busy.set(false);
    }
  }
}

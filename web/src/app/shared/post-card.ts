import { ChangeDetectionStrategy, Component, computed, inject, input } from '@angular/core';
import { RouterLink } from '@angular/router';
import type { PostCard as Card } from '../core/api.types';
import { BoardStore } from '../core/board.store';
import { I18n } from '../core/i18n/i18n';
import { postUrl } from '../core/route-utils';
import { Avatar } from './avatar';
import { Icon } from './icon';
import { StatusChip } from './status-chip';
import { VoteButton } from './vote-button';

@Component({
  selector: 'pb-post-card',
  imports: [RouterLink, Avatar, Icon, StatusChip, VoteButton],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @let p = post();
    <article class="card" data-testid="post-card">
      <h3 class="title">
        @if (p.pinned) {<pb-icon name="pin" class="pin" [attr.title]="i18n.t('post.pinned')" />}
        <a [routerLink]="url()" queryParamsHandling="preserve" class="stretch">{{ p.title }}</a>
      </h3>
      @if (p.excerpt) {<p class="excerpt">{{ p.excerpt }}</p>}
      @if (tag() || showStatus() || p.pending) {
        <div class="chips">
          @if (showStatus()) {<pb-status-chip [statusId]="p.statusId" />}
          @if (tag(); as t) {<span class="chip">{{ i18n.name(t.name) }}</span>}
          @if (p.pending) {<span class="chip">{{ i18n.t('post.pending') }}</span>}
        </div>
      }
      @if (p.newVotes >= 2) {
        <p class="trend"><pb-icon name="trend" />{{ i18n.plural('post.newVotes', p.newVotes) }}</p>
      }
      <div class="foot">
        <pb-vote-button [postId]="p.id" [count]="p.voteCount" [voted]="p.voted" />
        @if (p.commentCount > 0) {
          <span class="comments" [attr.aria-label]="i18n.plural('post.comments', p.commentCount)"><pb-icon name="comment" />{{ p.commentCount }}</span>
        }
        <span class="voters" aria-hidden="true">
          @for (v of p.voters; track v.id) {<pb-avatar [seed]="v.id" [size]="22" />}
        </span>
      </div>
    </article>
  `,
  styles: `
    .card { position: relative; padding: 16px 18px; border-radius: var(--pb-radius); border: 1px solid var(--pb-border);
      background: var(--pb-surface); box-shadow: var(--pb-shadow); transition: border-color .12s; }
    .card:hover { border-color: color-mix(in srgb, var(--pb-text) 22%, var(--pb-border)); }
    .title { margin: 0; font-size: 16px; font-weight: 500; line-height: 1.4; overflow-wrap: anywhere; }
    .title a { text-decoration: none; }
    .stretch::after { content: ''; position: absolute; inset: 0; border-radius: inherit; }
    .pin { width: 15px; height: 15px; color: var(--pb-accent); vertical-align: -2px; margin-right: 4px; }
    .excerpt { margin: 4px 0 0; color: var(--pb-muted); font-size: 14px; display: -webkit-box; -webkit-line-clamp: 2;
      -webkit-box-orient: vertical; overflow: hidden; }
    .chips { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 10px; }
    .trend { display: flex; align-items: center; gap: 6px; margin: 10px 0 0; font-size: 13px; font-weight: 600; }
    .trend pb-icon { width: 16px; height: 16px; }
    .foot { display: flex; align-items: center; gap: 12px; margin-top: 12px; }
    .comments { display: inline-flex; align-items: center; gap: 4px; color: var(--pb-muted); font-size: 13px; }
    .comments pb-icon { width: 16px; height: 16px; }
    .voters { display: inline-flex; margin-left: auto; }
    .voters pb-avatar + pb-avatar { margin-left: -6px; }
  `,
})
export class PostCard {
  readonly post = input.required<Card>();
  readonly showStatus = input(false);
  protected readonly i18n = inject(I18n);
  private readonly board = inject(BoardStore);
  protected readonly tag = computed(() => this.board.tag(this.post().tagId));
  protected readonly url = computed(() => postUrl(this.post()));
}

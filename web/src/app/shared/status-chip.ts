import { ChangeDetectionStrategy, Component, computed, inject, input } from '@angular/core';
import { BoardStore } from '../core/board.store';
import { I18n } from '../core/i18n/i18n';

@Component({
  selector: 'pb-status-chip',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (status(); as s) {
      <span class="chip" data-testid="status-chip"><span class="dot" [style.background]="s.color"></span>{{ i18n.name(s.name) }}</span>
    }
  `,
  styles: `
    .chip { display: inline-flex; align-items: center; gap: 6px; padding: 4px 10px; border-radius: 999px;
      background: var(--pb-chip); color: var(--pb-text); font-size: 13px; white-space: nowrap; }
    .dot { width: 8px; height: 8px; border-radius: 50%; flex: none; }
  `,
})
export class StatusChip {
  readonly statusId = input.required<number>();
  protected readonly i18n = inject(I18n);
  private readonly board = inject(BoardStore);
  protected readonly status = computed(() => this.board.status(this.statusId()));
}

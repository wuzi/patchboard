import { Injectable, computed, inject, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { Api } from './api';
import type { Board, Status, Tag } from './api.types';

/** The board settings, statuses and tags, loaded once at startup (reload after admin edits). */
@Injectable({ providedIn: 'root' })
export class BoardStore {
  private readonly api = inject(Api);
  readonly board = signal<Board | null>(null);
  private readonly statusMap = computed(() => new Map((this.board()?.statuses ?? []).map((s) => [s.id, s])));
  private readonly tagMap = computed(() => new Map((this.board()?.tags ?? []).map((t) => [t.id, t])));
  readonly statuses = computed(() => this.board()?.statuses ?? []);
  readonly tags = computed(() => this.board()?.tags ?? []);
  readonly defaultStatus = computed(() => this.statuses().find((s) => s.isDefault) ?? null);

  async load(): Promise<void> {
    try {
      const b = await firstValueFrom(this.api.board());
      this.board.set(b);
      document.documentElement.style.setProperty('--pb-accent', b.accentColor);
    } catch {
      /* the shell shows its own error state when the board is missing */
    }
  }

  status(id: number | null | undefined): Status | undefined {
    return id == null ? undefined : this.statusMap().get(id);
  }

  tag(id: number | null | undefined): Tag | undefined {
    return id == null ? undefined : this.tagMap().get(id);
  }
}

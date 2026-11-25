import { Injectable, signal } from '@angular/core';
import { Subject } from 'rxjs';
import type { VoteState } from './api.types';

/** Cross-view notifications: a vote cast in the modal updates the card behind it, and edits refresh lists. */
@Injectable({ providedIn: 'root' })
export class BoardEvents {
  readonly votes = new Subject<{ postId: number; state: VoteState }>();
  /** Bumped whenever posts change in a way lists should reload for (create, delete, status change…). */
  readonly revision = signal(0);
  /** Whether the "new post" dialog is open (any view can open it). */
  readonly newPostOpen = signal(false);

  changed(): void {
    this.revision.update((n) => n + 1);
  }
}

import { Injectable, signal } from '@angular/core';

export type BoardView = 'list' | 'roadmap' | 'updates';

/** Remembers which board view sits behind the post modal and how to get back to it. */
@Injectable({ providedIn: 'root' })
export class NavState {
  readonly lastView = signal<BoardView>('list');
  readonly lastUrl = signal('/');
}

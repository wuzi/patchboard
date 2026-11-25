import { ChangeDetectionStrategy, Component } from '@angular/core';

/** Placeholder; the board shell is built in the next task. */
@Component({
  selector: 'pb-board-shell',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `<main class="page"></main>`,
})
export class BoardShell {}

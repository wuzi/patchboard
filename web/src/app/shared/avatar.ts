import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

const PALETTE = ['#f97316', '#0ea5e9', '#a855f7', '#22c55e', '#ef4444', '#eab308', '#14b8a6', '#ec4899'];

function fnv1a(s: string): number {
  let h = 0x811c9dc5;
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 0x01000193) >>> 0;
  }
  return h >>> 0;
}

/** A 5×5 mirrored identicon: the colour and the 15 independent cells come from the seed's hash. */
export function avatarModel(seed: string): { color: string; cells: { x: number; y: number }[] } {
  let h = fnv1a(seed || '?');
  const color = PALETTE[h % PALETTE.length];
  const cells: { x: number; y: number }[] = [];
  for (let y = 0; y < 5; y++) {
    for (let x = 0; x < 3; x++) {
      h = Math.imul(h ^ (h >>> 15), 0x2c1b3c6d) >>> 0;
      if (h & 1) {
        cells.push({ x, y });
        if (x < 2) cells.push({ x: 4 - x, y });
      }
    }
  }
  return { color, cells };
}

export function avatarSvg(seed: string): string {
  const m = avatarModel(seed);
  const rects = m.cells.map((c) => `<rect x="${2 + c.x * 4}" y="${2 + c.y * 4}" width="4" height="4"/>`).join('');
  return `<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="12" fill="${m.color}"/><g fill="#fff" opacity=".9">${rects}</g></svg>`;
}

@Component({
  selector: 'pb-avatar',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <svg [attr.width]="size()" [attr.height]="size()" viewBox="0 0 24 24" aria-hidden="true">
      <circle cx="12" cy="12" r="12" [attr.fill]="model().color" />
      <g fill="#fff" opacity=".9">
        @for (c of model().cells; track $index) {
          <rect [attr.x]="2 + c.x * 4" [attr.y]="2 + c.y * 4" width="4" height="4" />
        }
      </g>
    </svg>
  `,
  styles: `:host { display: inline-flex; border-radius: 50%; box-shadow: 0 0 0 2px var(--pb-surface); } svg { display: block; }`,
})
export class Avatar {
  readonly seed = input.required<string>();
  readonly size = input(24);
  protected readonly model = computed(() => avatarModel(this.seed()));
}

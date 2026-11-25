import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

// Simple 24×24 stroke icons (drawn for PatchBoard).
const PATHS: Record<string, string> = {
  search: 'M11 4a7 7 0 1 1 0 14 7 7 0 0 1 0-14zM20 20l-4-4',
  filter: 'M4 6h16M7 12h10M10 18h4',
  plus: 'M12 5v14M5 12h14',
  list: 'M9 6h11M9 12h11M9 18h11M4.5 6h.01M4.5 12h.01M4.5 18h.01',
  roadmap: 'M5 4v16M12 4v9M19 4v12',
  updates: 'M5 4h11a2 2 0 0 1 2 2v14H7a2 2 0 0 1-2-2zM18 9h1.5a.5.5 0 0 1 .5.5V18a2 2 0 0 1-2 2M9 8h5M9 12h5M9 16h3',
  up: 'M7 14l5-5 5 5',
  comment: 'M5 5h14a1 1 0 0 1 1 1v10a1 1 0 0 1-1 1h-7l-4 3v-3H5a1 1 0 0 1-1-1V6a1 1 0 0 1 1-1z',
  trend: 'M4 17l6-6 4 4 6-7M14 8h6v6',
  pin: 'M9 4h6l-1 6 3 3H7l3-3zM12 16v5',
  link: 'M10 14a4 4 0 0 0 5.66 0l3-3a4 4 0 0 0-5.66-5.66l-1 1M14 10a4 4 0 0 0-5.66 0l-3 3a4 4 0 0 0 5.66 5.66l1-1',
  close: 'M6 6l12 12M18 6L6 18',
  heart: 'M12 20s-7-4.5-7-10a4 4 0 0 1 7-2.6A4 4 0 0 1 19 10c0 5.5-7 10-7 10z',
  bold: 'M7 5h6a3.5 3.5 0 0 1 0 7H7zM7 12h7a3.5 3.5 0 0 1 0 7H7z',
  italic: 'M14 5h-4M14 19h-4M14 5l-4 14M18 5h-4M10 19H6',
  bullet: 'M9 7h11M9 12h11M9 17h11M4.5 7h.01M4.5 12h.01M4.5 17h.01',
  number: 'M10 7h10M10 12h10M10 17h10M4 6l1.5-1V9M4 14.5a1.5 1.5 0 0 1 3 0c0 1.5-3 2-3 3.5h3',
  heading: 'M6 5v14M18 5v14M6 12h12',
  code: 'M9 8l-4 4 4 4M15 8l4 4-4 4',
  image: 'M5 5h14v14H5zM5 15l4-4 4 4 2-2 4 4M14.5 9.5h.01',
  trash: 'M5 7h14M10 7V5h4v2M7 7l1 13h8l1-13',
  edit: 'M5 19h4L19 9l-4-4L5 15zM13.5 6.5l4 4',
  lock: 'M7 11V8a5 5 0 0 1 10 0v3M6 11h12v9H6z',
  unlock: 'M7 11V8a5 5 0 0 1 9.5-2M6 11h12v9H6z',
  merge: 'M7 4v4a5 5 0 0 0 5 5h5M14 10l3 3-3 3M7 20v-4',
  check: 'M5 12l5 5 9-10',
  user: 'M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM5 20a7 7 0 0 1 14 0',
  globe: 'M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zM3 12h18M12 3c2.5 2.5 3.5 5.5 3.5 9s-1 6.5-3.5 9c-2.5-2.5-3.5-5.5-3.5-9s1-6.5 3.5-9z',
  copy: 'M9 9h10v10H9zM5 15V5h10',
  ban: 'M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zM5.6 5.6l12.8 12.8',
  restore: 'M4 12a8 8 0 1 0 2.3-5.7M4 4v5h5',
  logout: 'M15 4h4v16h-4M10 8l-4 4 4 4M6 12h10',
  settings: 'M12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6zM12 2v3M12 19v3M2 12h3M19 12h3M4.9 4.9l2.1 2.1M17 17l2.1 2.1M4.9 19.1L7 17M17 7l2.1-2.1',
  grip: 'M9 6h.01M15 6h.01M9 12h.01M15 12h.01M9 18h.01M15 18h.01',
  x: 'M6 6l12 12M18 6L6 18',
};

@Component({
  selector: 'pb-icon',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" [attr.stroke-width]="weight()"
    stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path [attr.d]="d()" /></svg>`,
  styles: `:host { display: inline-flex; width: 18px; height: 18px; flex: none; } svg { width: 100%; height: 100%; }`,
})
export class Icon {
  readonly name = input.required<string>();
  readonly weight = input(1.8);
  protected readonly d = computed(() => PATHS[this.name()] ?? '');
}

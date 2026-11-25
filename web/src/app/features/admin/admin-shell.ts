import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { I18n, type TKey } from '../../core/i18n/i18n';
import { Icon } from '../../shared/icon';

@Component({
  selector: 'pb-admin-shell',
  imports: [RouterLink, RouterLinkActive, RouterOutlet, Icon],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="admin">
      <nav class="side">
        <a class="back" routerLink="/"><pb-icon name="up" class="rot" />{{ i18n.t('admin.backToBoard') }}</a>
        @for (l of links; track l.path) {
          <a [routerLink]="l.path" routerLinkActive="active">{{ i18n.t(l.label) }}</a>
        }
      </nav>
      <main class="content"><router-outlet /></main>
    </div>
  `,
  styles: `
    .admin { display: grid; grid-template-columns: 220px 1fr; gap: 24px; max-width: 1100px; margin: 0 auto; padding: 32px 16px 64px; }
    .side { display: flex; flex-direction: column; gap: 2px; position: sticky; top: 24px; align-self: start; }
    .side a { padding: 8px 12px; border-radius: 10px; color: var(--pb-muted); text-decoration: none; font-weight: 500; }
    .side a:hover { background: var(--pb-hover); color: var(--pb-text); }
    .side a.active { background: var(--pb-chip); color: var(--pb-text); }
    .side .back { display: inline-flex; align-items: center; gap: 6px; margin-bottom: 12px; }
    .rot { transform: rotate(-90deg); }
    .content { min-width: 0; }
    @media (max-width: 760px) {
      .admin { grid-template-columns: 1fr; }
      .side { position: static; flex-direction: row; flex-wrap: wrap; }
      .side .back { width: 100%; }
    }
  `,
})
export class AdminShell {
  protected readonly i18n = inject(I18n);
  protected readonly links: { path: string; label: TKey }[] = [
    { path: 'settings', label: 'admin.navBoard' },
    { path: 'statuses', label: 'admin.navStatuses' },
    { path: 'tags', label: 'admin.navTags' },
    { path: 'team', label: 'admin.navTeam' },
    { path: 'moderation', label: 'admin.navModeration' },
  ];
}

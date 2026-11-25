import { Routes } from '@angular/router';
import { adminGuard } from './admin-guard';
import { AdminLogin } from './admin-login';
import { AdminShell } from './admin-shell';
import { BoardSettings } from './board-settings';
import { InviteAccept } from './invite-accept';
import { StatusesPage } from './statuses-page';
import { TagsPage } from './tags-page';
import { TeamPage } from './team-page';

export const adminRoutes: Routes = [
  { path: 'login', component: AdminLogin },
  { path: 'invite/:token', component: InviteAccept },
  {
    path: '',
    component: AdminShell,
    canActivate: [adminGuard],
    children: [
      { path: '', pathMatch: 'full', redirectTo: 'settings' },
      { path: 'settings', component: BoardSettings },
      { path: 'statuses', component: StatusesPage },
      { path: 'tags', component: TagsPage },
      { path: 'team', component: TeamPage },
    ],
  },
];

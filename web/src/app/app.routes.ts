import { Routes } from '@angular/router';
import { BoardShell } from './features/board/board-shell';

const shell = (view: 'list' | 'roadmap' | 'updates' | 'post') => ({ view, shell: 'board' });

export const routes: Routes = [
  { path: '', component: BoardShell, data: shell('list') },
  { path: 'roadmap', component: BoardShell, data: shell('roadmap') },
  { path: 'updates', component: BoardShell, data: shell('updates') },
  { path: 'p/:idSlug', component: BoardShell, data: shell('post') },
  { path: 'admin', loadChildren: () => import('./features/admin/admin.routes').then((m) => m.adminRoutes) },
  { path: '**', redirectTo: '' },
];

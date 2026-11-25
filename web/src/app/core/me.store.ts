import { Injectable, computed, inject, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { Api } from './api';
import type { Me } from './api.types';

/** Who the visitor is: a guest identity (after their first action) and/or a team session. */
@Injectable({ providedIn: 'root' })
export class MeStore {
  private readonly api = inject(Api);
  readonly me = signal<Me>({ guest: null, admin: null });
  readonly isAdmin = computed(() => this.me().admin !== null);

  async refresh(): Promise<void> {
    try {
      this.me.set(await firstValueFrom(this.api.me()));
    } catch {
      /* keep the previous value */
    }
  }

  set(me: Me): void {
    this.me.set(me);
  }
}

import { CdkDrag, CdkDragDrop, CdkDragHandle, CdkDropList, moveItemInArray } from '@angular/cdk/drag-drop';
import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { Api } from '../../core/api';
import { ApiError } from '../../core/api-error';
import type { Status } from '../../core/api.types';
import { BoardStore } from '../../core/board.store';
import { I18n } from '../../core/i18n/i18n';
import { Toast } from '../../core/toast';
import { Icon } from '../../shared/icon';

@Component({
  selector: 'pb-statuses-page',
  imports: [CdkDropList, CdkDrag, CdkDragHandle, Icon],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <h1 class="admin-title">{{ i18n.t('admin.navStatuses') }}</h1>
    <div class="rows" cdkDropList (cdkDropListDropped)="reorder($event)">
      @for (s of store.statuses(); track s.id) {
        <div class="row-item" cdkDrag data-testid="status-row">
          <span class="grip" cdkDragHandle><pb-icon name="grip" /></span>
          <input type="color" [value]="s.color" [attr.aria-label]="i18n.t('admin.color')" (change)="update(s, { color: $any($event.target).value })" />
          <input class="input" maxlength="30" [attr.aria-label]="i18n.t('admin.namePt')" [value]="s.name.pt"
            (change)="update(s, { name: { pt: $any($event.target).value, en: s.name.en } })" />
          <input class="input" maxlength="30" [attr.aria-label]="i18n.t('admin.nameEn')" [value]="s.name.en"
            (change)="update(s, { name: { pt: s.name.pt, en: $any($event.target).value } })" />
          <label class="check inline"><input type="checkbox" [checked]="s.showOnRoadmap" (change)="update(s, { showOnRoadmap: $any($event.target).checked })" />{{ i18n.t('admin.showOnRoadmap') }}</label>
          <label class="check inline"><input type="radio" name="default" [checked]="s.isDefault" (change)="update(s, { isDefault: true })" />{{ i18n.t('admin.default') }}</label>
          <button type="button" class="icon-btn" [attr.aria-label]="i18n.t('common.delete')" [disabled]="s.isDefault" (click)="remove(s)"><pb-icon name="trash" /></button>
          @if (moving() === s.id) {
            <div class="move">
              <label class="field"><span>{{ i18n.t('admin.moveTo') }}</span>
                <select class="input" #target>
                  @for (o of store.statuses(); track o.id) { @if (o.id !== s.id) {<option [value]="o.id">{{ i18n.name(o.name) }}</option>} }
                </select></label>
              <button type="button" class="btn danger" (click)="remove(s, +target.value)">{{ i18n.t('common.delete') }}</button>
              <button type="button" class="btn ghost" (click)="moving.set(null)">{{ i18n.t('common.cancel') }}</button>
            </div>
          }
        </div>
      }
    </div>
    <form class="panel-card new" data-testid="new-status" (submit)="$event.preventDefault(); create()">
      <h2>{{ i18n.t('admin.newStatus') }}</h2>
      <div class="row-item flat">
        <input type="color" [value]="color()" [attr.aria-label]="i18n.t('admin.color')" (input)="color.set($any($event.target).value)" />
        <input class="input" maxlength="30" [attr.aria-label]="i18n.t('admin.namePt')" [value]="pt()" (input)="pt.set($any($event.target).value)" />
        <input class="input" maxlength="30" [attr.aria-label]="i18n.t('admin.nameEn')" [value]="en()" (input)="en.set($any($event.target).value)" />
        <button type="submit" class="btn primary" [disabled]="!pt().trim() || !en().trim()">{{ i18n.t('common.add') }}</button>
      </div>
    </form>
  `,
  styles: `
    .check.inline { margin: 0; font-size: 13px; }
    .move { display: flex; align-items: flex-end; gap: 8px; width: 100%; flex-wrap: wrap; }
    .move .field { margin: 0; flex: 1; }
    .new { margin-top: 20px; }
    .row-item.flat { border: 0; padding: 0; }
  `,
})
export class StatusesPage {
  protected readonly i18n = inject(I18n);
  protected readonly store = inject(BoardStore);
  private readonly api = inject(Api);
  private readonly toast = inject(Toast);
  protected readonly moving = signal<number | null>(null);
  protected readonly color = signal('#64748B');
  protected readonly pt = signal('');
  protected readonly en = signal('');

  private async run(op: () => Promise<unknown>): Promise<boolean> {
    try {
      await op();
      await this.store.load();
      return true;
    } catch (e) {
      this.toast.error(e);
      await this.store.load();
      return false;
    }
  }

  protected update(s: Status, patch: Parameters<Api['updateStatus']>[1]): void {
    this.run(() => firstValueFrom(this.api.updateStatus(s.id, patch)));
  }

  protected reorder(e: CdkDragDrop<unknown>): void {
    const ids = this.store.statuses().map((s) => s.id);
    moveItemInArray(ids, e.previousIndex, e.currentIndex);
    this.run(() => firstValueFrom(this.api.orderStatuses(ids)));
  }

  protected async remove(s: Status, moveTo?: number): Promise<void> {
    try {
      await firstValueFrom(this.api.deleteStatus(s.id, moveTo));
      this.moving.set(null);
      await this.store.load();
    } catch (e) {
      if (e instanceof ApiError && e.code === 'move_required') this.moving.set(s.id);
      else this.toast.error(e);
    }
  }

  protected async create(): Promise<void> {
    const ok = await this.run(() =>
      firstValueFrom(this.api.createStatus({ name: { pt: this.pt().trim(), en: this.en().trim() }, color: this.color(), showOnRoadmap: true })),
    );
    if (ok) {
      this.pt.set('');
      this.en.set('');
    }
  }
}

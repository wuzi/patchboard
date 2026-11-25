import { CdkDrag, CdkDragDrop, CdkDragHandle, CdkDropList, moveItemInArray } from '@angular/cdk/drag-drop';
import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { Api } from '../../core/api';
import type { Tag } from '../../core/api.types';
import { BoardStore } from '../../core/board.store';
import { I18n } from '../../core/i18n/i18n';
import { Toast } from '../../core/toast';
import { Icon } from '../../shared/icon';

@Component({
  selector: 'pb-tags-page',
  imports: [CdkDropList, CdkDrag, CdkDragHandle, Icon],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <h1 class="admin-title">{{ i18n.t('admin.navTags') }}</h1>
    <div class="rows" cdkDropList (cdkDropListDropped)="reorder($event)">
      @for (t of store.tags(); track t.id) {
        <div class="row-item" cdkDrag>
          <span class="grip" cdkDragHandle><pb-icon name="grip" /></span>
          <input class="input" maxlength="30" [attr.aria-label]="i18n.t('admin.namePt')" [value]="t.name.pt" (change)="rename(t, $any($event.target).value, t.name.en)" />
          <input class="input" maxlength="30" [attr.aria-label]="i18n.t('admin.nameEn')" [value]="t.name.en" (change)="rename(t, t.name.pt, $any($event.target).value)" />
          <button type="button" class="icon-btn" [attr.aria-label]="i18n.t('common.delete')" (click)="remove(t)"><pb-icon name="trash" /></button>
        </div>
      }
    </div>
    <form class="panel-card new" (submit)="$event.preventDefault(); create()">
      <h2>{{ i18n.t('admin.newTag') }}</h2>
      <div class="row-item flat">
        <input class="input" maxlength="30" [attr.aria-label]="i18n.t('admin.namePt')" [value]="pt()" (input)="pt.set($any($event.target).value)" />
        <input class="input" maxlength="30" [attr.aria-label]="i18n.t('admin.nameEn')" [value]="en()" (input)="en.set($any($event.target).value)" />
        <button type="submit" class="btn primary" [disabled]="!pt().trim() || !en().trim()">{{ i18n.t('common.add') }}</button>
      </div>
    </form>
  `,
  styles: `.new { margin-top: 20px; } .row-item.flat { border: 0; padding: 0; }`,
})
export class TagsPage {
  protected readonly i18n = inject(I18n);
  protected readonly store = inject(BoardStore);
  private readonly api = inject(Api);
  private readonly toast = inject(Toast);
  protected readonly pt = signal('');
  protected readonly en = signal('');

  private async run(op: () => Promise<unknown>): Promise<boolean> {
    try {
      await op();
      return true;
    } catch (e) {
      this.toast.error(e);
      return false;
    } finally {
      await this.store.load();
    }
  }

  protected rename(t: Tag, pt: string, en: string): void {
    this.run(() => firstValueFrom(this.api.updateTag(t.id, { pt, en })));
  }

  protected reorder(e: CdkDragDrop<unknown>): void {
    const ids = this.store.tags().map((t) => t.id);
    moveItemInArray(ids, e.previousIndex, e.currentIndex);
    this.run(() => firstValueFrom(this.api.orderTags(ids)));
  }

  protected remove(t: Tag): void {
    if (confirm(this.i18n.t('common.delete') + ': ' + this.i18n.name(t.name) + '?')) this.run(() => firstValueFrom(this.api.deleteTag(t.id)));
  }

  protected async create(): Promise<void> {
    if (await this.run(() => firstValueFrom(this.api.createTag({ pt: this.pt().trim(), en: this.en().trim() })))) {
      this.pt.set('');
      this.en.set('');
    }
  }
}

import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { Api } from '../../core/api';
import type { Image, Settings } from '../../core/api.types';
import { BoardStore } from '../../core/board.store';
import { I18n } from '../../core/i18n/i18n';
import { Toast } from '../../core/toast';
import { ImagePicker } from '../../shared/image-picker';
import { fieldMessage, fieldsOf } from './field-errors';

@Component({
  selector: 'pb-board-settings',
  imports: [ImagePicker],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <h1 class="admin-title">{{ i18n.t('admin.navBoard') }}</h1>
    <form class="panel-card" (submit)="$event.preventDefault(); save()">
      <div class="grid-2">
        <label class="field"><span>{{ i18n.t('admin.titlePt') }}</span>
          <input class="input" maxlength="80" [value]="s().title.pt" (input)="set('title', 'pt', $event)" /><span class="field-error">{{ msg('title.pt') }}</span></label>
        <label class="field"><span>{{ i18n.t('admin.titleEn') }}</span>
          <input class="input" maxlength="80" [value]="s().title.en" (input)="set('title', 'en', $event)" /><span class="field-error">{{ msg('title.en') }}</span></label>
        <label class="field"><span>{{ i18n.t('admin.descriptionPt') }}</span>
          <textarea class="input" rows="3" maxlength="500" [value]="s().description.pt" (input)="set('description', 'pt', $event)"></textarea></label>
        <label class="field"><span>{{ i18n.t('admin.descriptionEn') }}</span>
          <textarea class="input" rows="3" maxlength="500" [value]="s().description.en" (input)="set('description', 'en', $event)"></textarea></label>
        <label class="field"><span>{{ i18n.t('admin.teamName') }}</span>
          <input class="input" maxlength="40" [value]="s().teamName" (input)="patch({ teamName: $any($event.target).value })" /><span class="field-error">{{ msg('teamName') }}</span></label>
        <label class="field"><span>{{ i18n.t('admin.accent') }}</span>
          <span class="accent"><input type="color" [value]="s().accentColor" (input)="patch({ accentColor: $any($event.target).value })" />
            <input class="input" maxlength="7" [value]="s().accentColor" (input)="patch({ accentColor: $any($event.target).value })" /></span>
          <span class="field-error">{{ msg('accentColor') }}</span></label>
      </div>
      <div class="field"><span>{{ i18n.t('admin.logo') }}</span><pb-image-picker [max]="1" [images]="logo()" (imagesChange)="setLogo($event)" /></div>
      <label class="check"><input type="checkbox" [checked]="s().requireApproval" (change)="patch({ requireApproval: $any($event.target).checked })" />{{ i18n.t('admin.requireApproval') }}</label>
      <label class="check"><input type="checkbox" [checked]="s().uploadsEnabled" (change)="patch({ uploadsEnabled: $any($event.target).checked })" />{{ i18n.t('admin.uploadsEnabled') }}</label>
      <div class="actions-end"><button type="submit" class="btn primary" [disabled]="busy()">{{ i18n.t('common.save') }}</button></div>
    </form>
  `,
  styles: `.accent { display: flex; gap: 8px; } .accent input[type=color] { width: 42px; height: 40px; padding: 2px; border: 1px solid var(--pb-border); border-radius: 10px; background: none; }`,
})
export class BoardSettings {
  protected readonly i18n = inject(I18n);
  private readonly store = inject(BoardStore);
  private readonly api = inject(Api);
  private readonly toast = inject(Toast);
  protected readonly busy = signal(false);
  private readonly errors = signal<Record<string, string>>({});
  protected readonly s = signal<Settings>(this.fromBoard());
  protected readonly logo = signal<Image[]>(this.logoImages());

  private fromBoard(): Settings {
    const b = this.store.board()!;
    const logoId = b.logoUrl ? b.logoUrl.split('/').pop()!.split('.')[0] : null;
    return {
      title: { ...b.title }, description: { ...b.description }, teamName: b.teamName, accentColor: b.accentColor,
      logoUploadId: logoId, requireApproval: b.requireApproval, uploadsEnabled: b.uploadsEnabled,
    };
  }

  private logoImages(): Image[] {
    const b = this.store.board();
    const id = this.s?.()?.logoUploadId;
    return b?.logoUrl && id ? [{ id, url: b.logoUrl, width: 0, height: 0 }] : [];
  }

  protected msg(f: string): string {
    return fieldMessage(this.i18n, this.errors()[f]);
  }

  protected patch(p: Partial<Settings>): void {
    this.s.update((v) => ({ ...v, ...p }));
  }

  protected set(field: 'title' | 'description', lang: 'pt' | 'en', e: Event): void {
    const value = (e.target as HTMLInputElement).value;
    this.s.update((v) => ({ ...v, [field]: { ...v[field], [lang]: value } }));
  }

  protected setLogo(images: Image[]): void {
    this.logo.set(images);
    this.patch({ logoUploadId: images[0]?.id ?? null });
  }

  protected async save(): Promise<void> {
    this.busy.set(true);
    this.errors.set({});
    try {
      const b = await firstValueFrom(this.api.saveSettings(this.s()));
      this.store.board.set(b);
      document.documentElement.style.setProperty('--pb-accent', b.accentColor);
      this.toast.show(this.i18n.t('common.saved'));
    } catch (e) {
      const f = fieldsOf(e);
      if (Object.keys(f).length) this.errors.set(f);
      this.toast.error(e);
    } finally {
      this.busy.set(false);
    }
  }
}

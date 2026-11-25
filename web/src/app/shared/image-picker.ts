import { ChangeDetectionStrategy, Component, computed, inject, input, model, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { Api } from '../core/api';
import type { Image } from '../core/api.types';
import { I18n } from '../core/i18n/i18n';
import { Toast } from '../core/toast';
import { Icon } from './icon';

@Component({
  selector: 'pb-image-picker',
  imports: [Icon],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="grid">
      @for (img of images(); track img.id; let i = $index) {
        <figure class="thumb">
          <img [src]="img.url" [alt]="i18n.t('post.imageAlt', { n: i + 1 })" />
          <button type="button" class="remove" [attr.aria-label]="i18n.t('post.removeImage')" (click)="remove(img)"><pb-icon name="x" /></button>
        </figure>
      }
      @for (n of pendingSlots(); track $index) {<div class="thumb loading" aria-busy="true"></div>}
      @if (canAdd()) {
        <button type="button" class="add" (click)="file.click()"><pb-icon name="image" /><span>{{ i18n.t('post.addImages') }}</span></button>
      }
    </div>
    <input #file type="file" accept="image/png,image/jpeg,image/gif,image/webp" [multiple]="max() > 1" hidden (change)="picked($event)" />
  `,
  styles: `
    .grid { display: flex; flex-wrap: wrap; gap: 8px; }
    .thumb { position: relative; width: 88px; height: 88px; margin: 0; border-radius: 12px; overflow: hidden; border: 1px solid var(--pb-border); }
    .thumb img { width: 100%; height: 100%; object-fit: cover; display: block; }
    .thumb.loading { background: linear-gradient(90deg, var(--pb-column), var(--pb-hover), var(--pb-column)); background-size: 200% 100%;
      animation: shimmer 1.2s infinite linear; }
    @keyframes shimmer { to { background-position: -200% 0; } }
    .remove { position: absolute; top: 4px; right: 4px; display: inline-flex; padding: 3px; border: 0; border-radius: 999px;
      background: rgb(0 0 0 / .6); color: #fff; cursor: pointer; }
    .remove pb-icon { width: 14px; height: 14px; }
    .add { display: inline-flex; flex-direction: column; align-items: center; justify-content: center; gap: 4px; width: 88px; height: 88px;
      border: 1px dashed var(--pb-border); border-radius: 12px; background: transparent; color: var(--pb-muted); font-size: 11px; cursor: pointer; text-align: center; }
    .add:hover { background: var(--pb-hover); color: var(--pb-text); }
  `,
})
export class ImagePicker {
  readonly images = model<Image[]>([]);
  readonly max = input(4);

  protected readonly i18n = inject(I18n);
  private readonly api = inject(Api);
  private readonly toast = inject(Toast);
  private readonly uploading = signal(0);
  protected readonly pendingSlots = computed(() => Array.from({ length: this.uploading() }));
  protected readonly canAdd = computed(() => this.images().length + this.uploading() < this.max());

  protected async picked(e: Event): Promise<void> {
    const input = e.target as HTMLInputElement;
    const room = this.max() - this.images().length - this.uploading();
    const files = Array.from(input.files ?? []).slice(0, room);
    input.value = '';
    for (const f of files) {
      this.uploading.update((n) => n + 1);
      try {
        const img = await firstValueFrom(this.api.upload(f));
        this.images.update((l) => [...l, img]);
      } catch (err) {
        this.toast.error(err);
      } finally {
        this.uploading.update((n) => n - 1);
      }
    }
  }

  protected remove(img: Image): void {
    this.images.update((l) => l.filter((i) => i.id !== img.id));
  }
}

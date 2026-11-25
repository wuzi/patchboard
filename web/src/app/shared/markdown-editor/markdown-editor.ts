import { ChangeDetectionStrategy, Component, ElementRef, computed, inject, input, model, signal, viewChild } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { Api } from '../../core/api';
import { I18n, type TKey } from '../../core/i18n/i18n';
import { Icon } from '../icon';
import { applyFormat, type FormatKind } from './format';

@Component({
  selector: 'pb-markdown-editor',
  imports: [Icon],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="editor" [class.focus]="focused()">
      <div class="top">
        <div class="tabs" role="tablist">
          <button type="button" role="tab" [attr.aria-selected]="!previewing()" [class.on]="!previewing()" (click)="previewing.set(false)">{{ i18n.t('editor.write') }}</button>
          <button type="button" role="tab" [attr.aria-selected]="previewing()" [class.on]="previewing()" (click)="showPreview()">{{ i18n.t('editor.preview') }}</button>
        </div>
        @if (!previewing()) {
          <div class="tools">
            @for (b of buttons; track b.kind) {
              <button type="button" class="tool" [attr.aria-label]="i18n.t(b.label)" [attr.title]="i18n.t(b.label)"
                (mousedown)="$event.preventDefault()" (click)="format(b.kind)"><pb-icon [name]="b.icon" /></button>
            }
          </div>
        }
      </div>
      @if (previewing()) {
        @if (previewHtml() === null) {<p class="preview muted">{{ i18n.t('common.loading') }}</p>}
        @else if (previewHtml() === '') {<p class="preview muted">{{ i18n.t('editor.nothingToPreview') }}</p>}
        @else {<div class="preview md" [innerHTML]="previewHtml()"></div>}
      } @else {
        <textarea #area [rows]="rows()" [placeholder]="placeholder()" [value]="value()" [attr.maxlength]="maxLength()"
          [attr.aria-label]="label() || placeholder()" (input)="value.set($any($event.target).value)"
          (focus)="focused.set(true)" (blur)="focused.set(false)" (keydown)="shortcut($event)"></textarea>
      }
      @if (nearLimit()) {<p class="count">{{ value().length }} / {{ maxLength() }}</p>}
    </div>
  `,
  styles: `
    .editor { border: 1px solid var(--pb-border); border-radius: 14px; background: var(--pb-surface); overflow: hidden; }
    .editor.focus { border-color: var(--pb-accent); box-shadow: 0 0 0 3px color-mix(in srgb, var(--pb-accent) 18%, transparent); }
    .top { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 4px 6px;
      border-bottom: 1px solid var(--pb-border); flex-wrap: wrap; }
    .tabs { display: flex; gap: 2px; }
    .tabs button { padding: 4px 10px; border: 0; border-radius: 8px; background: transparent; color: var(--pb-muted);
      font-size: 13px; font-weight: 500; cursor: pointer; }
    .tabs button.on { background: var(--pb-chip); color: var(--pb-text); }
    .tools { display: flex; gap: 2px; flex-wrap: wrap; }
    .tool { display: inline-flex; padding: 6px; border: 0; border-radius: 8px; background: transparent; color: var(--pb-muted); cursor: pointer; }
    .tool:hover { background: var(--pb-hover); color: var(--pb-text); }
    .tool pb-icon { width: 16px; height: 16px; }
    textarea { display: block; width: 100%; min-height: 64px; padding: 12px 14px; border: 0; outline: none; resize: vertical;
      background: transparent; line-height: 1.5; }
    .preview { min-height: 64px; padding: 12px 14px; margin: 0; }
    .count { margin: 0; padding: 0 14px 8px; text-align: right; font-size: 12px; color: var(--pb-muted); }
  `,
})
export class MarkdownEditor {
  readonly value = model('');
  readonly placeholder = input('');
  readonly label = input('');
  readonly maxLength = input(10000);
  readonly rows = input(4);

  protected readonly i18n = inject(I18n);
  private readonly api = inject(Api);
  private readonly area = viewChild<ElementRef<HTMLTextAreaElement>>('area');
  protected readonly previewing = signal(false);
  protected readonly previewHtml = signal<string | null>(null);
  protected readonly focused = signal(false);
  protected readonly nearLimit = computed(() => this.value().length > this.maxLength() * 0.8);

  protected readonly buttons: { kind: FormatKind; icon: string; label: TKey }[] = [
    { kind: 'bold', icon: 'bold', label: 'editor.bold' },
    { kind: 'italic', icon: 'italic', label: 'editor.italic' },
    { kind: 'link', icon: 'link', label: 'editor.link' },
    { kind: 'bullet', icon: 'bullet', label: 'editor.bullet' },
    { kind: 'number', icon: 'number', label: 'editor.number' },
    { kind: 'heading', icon: 'heading', label: 'editor.heading' },
    { kind: 'code', icon: 'code', label: 'editor.code' },
  ];

  focus(): void {
    this.area()?.nativeElement.focus();
  }

  protected format(kind: FormatKind): void {
    const el = this.area()?.nativeElement;
    if (!el) return;
    const edit = applyFormat(this.value(), el.selectionStart, el.selectionEnd, kind);
    this.value.set(edit.text);
    el.value = edit.text;
    el.focus();
    el.setSelectionRange(edit.start, edit.end);
  }

  protected shortcut(e: KeyboardEvent): void {
    if (!(e.ctrlKey || e.metaKey)) return;
    const kind = e.key === 'b' ? 'bold' : e.key === 'i' ? 'italic' : e.key === 'k' ? 'link' : null;
    if (kind) {
      e.preventDefault();
      this.format(kind);
    }
  }

  protected async showPreview(): Promise<void> {
    this.previewing.set(true);
    if (!this.value().trim()) {
      this.previewHtml.set('');
      return;
    }
    this.previewHtml.set(null);
    try {
      this.previewHtml.set((await firstValueFrom(this.api.preview(this.value()))).html);
    } catch {
      this.previewHtml.set('');
    }
  }
}

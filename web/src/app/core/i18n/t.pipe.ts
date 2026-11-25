import { Pipe, PipeTransform, inject } from '@angular/core';
import { I18n, type TKey } from './i18n';

/** {{ 'nav.list' | t }} — impure so it follows language switches. */
@Pipe({ name: 't', pure: false })
export class TPipe implements PipeTransform {
  private readonly i18n = inject(I18n);

  transform(key: TKey, params?: Record<string, string | number>): string {
    return this.i18n.t(key, params);
  }
}

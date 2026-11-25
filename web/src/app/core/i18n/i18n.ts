import { Injectable, signal } from '@angular/core';
import type { I18nName } from '../api.types';
import { en } from './en';
import { pt, type TKey } from './pt';

export type Lang = 'pt' | 'en';
export type { TKey };

const STORAGE_KEY = 'pb_lang';
const dicts: Record<Lang, Record<TKey, string>> = { pt, en };
const locales: Record<Lang, string> = { pt: 'pt-BR', en: 'en' };

/** Browser language → UI language: anything Portuguese is pt, everything else en. */
export function detectLang(navigatorLang: string): Lang {
  return navigatorLang.toLowerCase().startsWith('pt') ? 'pt' : 'en';
}

function stored(): Lang | null {
  try {
    const v = localStorage.getItem(STORAGE_KEY);
    return v === 'pt' || v === 'en' ? v : null;
  } catch {
    return null;
  }
}

@Injectable({ providedIn: 'root' })
export class I18n {
  readonly lang = signal<Lang>(stored() ?? detectLang(typeof navigator === 'undefined' ? 'pt' : navigator.language));

  constructor() {
    this.applyHtmlLang(this.lang());
  }

  setLang(l: Lang): void {
    this.lang.set(l);
    try {
      localStorage.setItem(STORAGE_KEY, l);
    } catch {
      /* storage unavailable: the choice lasts for this page only */
    }
    this.applyHtmlLang(l);
  }

  get locale(): string {
    return locales[this.lang()];
  }

  has(key: string): key is TKey {
    return key in pt;
  }

  t(key: TKey, params?: Record<string, string | number>): string {
    let s = dicts[this.lang()][key] ?? key;
    if (params) {
      for (const [k, v] of Object.entries(params)) s = s.replaceAll(`{${k}}`, String(v));
    }
    return s;
  }

  /** plural('post.newVotes', 3) reads post.newVotes_one / post.newVotes_other with {n}. */
  plural(base: string, n: number): string {
    const form = new Intl.PluralRules(this.locale).select(n) === 'one' ? 'one' : 'other';
    return this.t(`${base}_${form}` as TKey, { n });
  }

  name(n: I18nName | null | undefined): string {
    return n ? n[this.lang()] : '';
  }

  date(iso: string, style: 'short' | 'long' = 'short'): string {
    const opts: Intl.DateTimeFormatOptions =
      style === 'long' ? { day: 'numeric', month: 'short', year: 'numeric' } : { day: 'numeric', month: 'short' };
    return new Intl.DateTimeFormat(this.locale, opts).format(new Date(iso));
  }

  private applyHtmlLang(l: Lang): void {
    if (typeof document !== 'undefined') document.documentElement.lang = locales[l];
  }
}

import { ApiError } from '../../core/api-error';
import type { I18n, TKey } from '../../core/i18n/i18n';

/** Translates API field reasons ("too_short") for display under inputs. */
export function fieldMessage(i18n: I18n, reason: string | undefined): string {
  if (!reason) return '';
  const key = `validation.${reason}`;
  return i18n.has(key) ? i18n.t(key as TKey) : reason;
}

export function fieldsOf(e: unknown): Record<string, string> {
  return e instanceof ApiError && e.code === 'validation' ? e.fields : {};
}

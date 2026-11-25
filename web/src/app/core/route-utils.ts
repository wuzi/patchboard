/** "12-aumentar-chat" → {id: 12, slug: "aumentar-chat"}; null when there is no positive id. */
export function parseIdSlug(s: string): { id: number; slug: string } | null {
  const m = /^(\d+)(?:-(.*))?$/.exec(s);
  if (!m) return null;
  const id = Number(m[1]);
  return id > 0 ? { id, slug: m[2] ?? '' } : null;
}

export function postUrl(p: { id: number; slug: string }): string {
  return `/p/${p.id}-${p.slug}`;
}

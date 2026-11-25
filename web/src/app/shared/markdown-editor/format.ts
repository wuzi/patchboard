export type FormatKind = 'bold' | 'italic' | 'link' | 'bullet' | 'number' | 'heading' | 'code';
export type Edit = { text: string; start: number; end: number };

function wrap(text: string, start: number, end: number, mark: string): Edit {
  const sel = text.slice(start, end);
  return { text: text.slice(0, start) + mark + sel + mark + text.slice(end), start: start + mark.length, end: end + mark.length };
}

/** Applies prefix(i) to every line touched by the selection; the result selects those lines. */
function prefixLines(text: string, start: number, end: number, prefix: (i: number) => string): Edit {
  const lineStart = text.lastIndexOf('\n', start - 1) + 1;
  let lineEnd = text.indexOf('\n', end);
  if (lineEnd === -1) lineEnd = text.length;
  const block = text
    .slice(lineStart, lineEnd)
    .split('\n')
    .map((l, i) => prefix(i) + l)
    .join('\n');
  return { text: text.slice(0, lineStart) + block + text.slice(lineEnd), start: lineStart, end: lineStart + block.length };
}

/** The Markdown a toolbar button inserts around/at the selection [start, end). */
export function applyFormat(text: string, start: number, end: number, kind: FormatKind): Edit {
  const sel = text.slice(start, end);
  switch (kind) {
    case 'bold':
      return wrap(text, start, end, '**');
    case 'italic':
      return wrap(text, start, end, '_');
    case 'code':
      if (sel.includes('\n')) {
        const block = '```\n' + sel + '\n```';
        return { text: text.slice(0, start) + block + text.slice(end), start: start + 4, end: start + 4 + sel.length };
      }
      return wrap(text, start, end, '`');
    case 'link': {
      const url = 'https://';
      const inserted = `[${sel}](${url})`;
      const urlStart = start + sel.length + 3;
      return { text: text.slice(0, start) + inserted + text.slice(end), start: urlStart, end: urlStart + url.length };
    }
    case 'bullet':
      return prefixLines(text, start, end, () => '- ');
    case 'number':
      return prefixLines(text, start, end, (i) => `${i + 1}. `);
    case 'heading':
      return prefixLines(text, start, end, () => '## ');
  }
}

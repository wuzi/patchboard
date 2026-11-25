import { applyFormat } from './format';

describe('applyFormat', () => {
  it('bolds a selection', () => expect(applyFormat('hello', 0, 5, 'bold')).toEqual({ text: '**hello**', start: 2, end: 7 }));
  it('bolds at the cursor', () => expect(applyFormat('ab', 1, 1, 'bold')).toEqual({ text: 'a****b', start: 3, end: 3 }));
  it('italicises with underscores', () => expect(applyFormat('x y', 2, 3, 'italic').text).toBe('x _y_'));
  it('bullets every selected line', () => expect(applyFormat('a\nb', 0, 3, 'bullet').text).toBe('- a\n- b'));
  it('numbers lines', () => expect(applyFormat('a\nb', 0, 3, 'number').text).toBe('1. a\n2. b'));
  it('prefixes the whole current line for a heading', () => expect(applyFormat('one\ntwo', 5, 5, 'heading').text).toBe('one\n## two'));
  it('links with the url selected', () =>
    expect(applyFormat('site', 0, 4, 'link')).toEqual({ text: '[site](https://)', start: 7, end: 15 }));
  it('fences multi-line code', () => expect(applyFormat('a\nb', 0, 3, 'code').text).toBe('```\na\nb\n```'));
  it('inlines single-line code', () => expect(applyFormat('run x', 4, 5, 'code').text).toBe('run `x`'));
});

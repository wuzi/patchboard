import { parseIdSlug, postUrl } from './route-utils';

describe('route utils', () => {
  it('parses id-slug', () => {
    expect(parseIdSlug('12-aumentar-chat')).toEqual({ id: 12, slug: 'aumentar-chat' });
    expect(parseIdSlug('12')).toEqual({ id: 12, slug: '' });
    expect(parseIdSlug('abc')).toBeNull();
    expect(parseIdSlug('0-x')).toBeNull();
  });

  it('builds post urls', () => {
    expect(postUrl({ id: 7, slug: 'co-op' })).toBe('/p/7-co-op');
  });
});

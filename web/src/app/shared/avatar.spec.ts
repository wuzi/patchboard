import { avatarSvg } from './avatar';

describe('avatar', () => {
  it('is deterministic', () => {
    expect(avatarSvg('k3j4')).toBe(avatarSvg('k3j4'));
    expect(avatarSvg('k3j4')).not.toBe(avatarSvg('zz99'));
    expect(avatarSvg('k3j4')).toContain('<svg');
  });
});

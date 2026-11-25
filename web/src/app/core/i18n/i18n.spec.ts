import { TestBed } from '@angular/core/testing';
import { I18n, detectLang } from './i18n';

describe('I18n', () => {
  let i18n: I18n;
  beforeEach(() => {
    localStorage.clear();
    i18n = TestBed.inject(I18n);
  });

  it('translates in both languages', () => {
    i18n.setLang('pt');
    expect(i18n.t('common.empty')).toBe('Nada aqui ainda');
    i18n.setLang('en');
    expect(i18n.t('common.empty')).toBe('Nothing here yet');
  });

  it('interpolates and pluralises', () => {
    i18n.setLang('en');
    expect(i18n.plural('post.newVotes', 4)).toBe('4 new votes this week');
    expect(i18n.plural('post.newVotes', 1)).toBe('1 new vote this week');
    expect(i18n.t('post.addedBy', { name: 'Wuzi', date: 'Nov 25' })).toBe('Added by Wuzi on Nov 25');
  });

  it('remembers the language and sets <html lang>', () => {
    i18n.setLang('en');
    expect(localStorage.getItem('pb_lang')).toBe('en');
    expect(document.documentElement.lang).toBe('en');
    i18n.setLang('pt');
    expect(document.documentElement.lang).toBe('pt-BR');
  });

  it('detects language', () => {
    expect(detectLang('pt-BR')).toBe('pt');
    expect(detectLang('pt-PT')).toBe('pt');
    expect(detectLang('en-US')).toBe('en');
    expect(detectLang('fr')).toBe('en');
  });

  it('formats dates per language', () => {
    i18n.setLang('pt');
    expect(i18n.date('2026-11-25T12:00:00Z', 'long')).toBe('25 de nov. de 2026');
    i18n.setLang('en');
    expect(i18n.date('2026-11-25T12:00:00Z', 'long')).toBe('Nov 25, 2026');
  });

  it('picks the name for the current language', () => {
    i18n.setLang('en');
    expect(i18n.name({ pt: 'Planejado', en: 'Planned' })).toBe('Planned');
  });
});

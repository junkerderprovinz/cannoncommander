import { afterAll, describe, expect, it } from 'vitest';
import { setLanguage } from '@/i18n';
import { fmtMeta } from './columns';

afterAll(() => setLanguage('de'));

describe('meta values in the interface language', () => {
  it('names words, counts and languages the server sends as codes', async () => {
    await setLanguage('de');
    expect(fmtMeta('portrait', 'word', 'orientation')).toBe('Hochformat');
    expect(fmtMeta('3', 'word', 'channels')).toBe('3 Kanäle');
    expect(fmtMeta('5.1', 'word', 'channels')).toBe('5.1');
    expect(fmtMeta('de, und', 'word', 'audiotracks')).toBe('Deutsch, Unbekannt');
    expect(fmtMeta('77', 'word', 'method')).toBe('Methode 77');
    expect(fmtMeta('Zstandard', 'word', 'method')).toBe('Zstandard');
    await setLanguage('en');
    expect(fmtMeta('fired', 'word', 'flash')).toBe('Fired');
    expect(fmtMeta('1', 'word', 'channels')).toBe('1 channel');
    expect(fmtMeta('de, en, tlh', 'word', 'subtitles')).toBe('German, English, Klingon');
    expect(fmtMeta('all', 'word', 'arch')).toBe('All');
  });

  it('writes decimal points the way the language does', async () => {
    await setLanguage('en');
    expect(fmtMeta('f/2.8', 'decimal')).toBe('f/2.8');
    await setLanguage('de');
    expect(fmtMeta('23.976 fps', 'decimal')).toBe('23,976 fps');
    expect(fmtMeta('1.85:1', 'decimal')).toBe('1,85:1');
  });
});

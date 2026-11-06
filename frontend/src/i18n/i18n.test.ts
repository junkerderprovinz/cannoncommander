import { afterAll, describe, expect, it } from 'vitest';
import '@/ui/texts';
import { browserLanguage, byName, language, plural, serverText, setLanguage, t, useLocales } from '.';

useLocales({
  './locales/ru.json': async () => ({ ui: { files: { one: '{n} файл', few: '{n} файла', many: '{n} файлов', other: '{n} файла' } } }),
  './locales/sv.json': async () => ({}),
});
afterAll(() => setLanguage('de'));

describe('the texts', () => {
  it('pick the plural form the language asks for', async () => {
    await setLanguage('ru');
    expect([1, 3, 5, 21, 22, 111].map((n) => plural(n, 'ui.files'))).toEqual(['1 файл', '3 файла', '5 файлов', '21 файл', '22 файла', '111 файлов']);
    expect(document.documentElement.lang).toBe('ru');
  });

  it('fall back to English for a text a language does not have yet', async () => {
    await setLanguage('ru');
    expect(t('ui.copy')).toBe('Copy');
    expect(plural(2, 'ui.objects')).toBe('2 items');
  });

  it('turn right to left for Arabic, Persian and Hebrew', async () => {
    useLocales({ './locales/he.json': async () => ({}) });
    await setLanguage('he');
    expect(document.documentElement.dir).toBe('rtl');
    await setLanguage('de');
    expect(document.documentElement.dir).toBe('ltr');
  });

  it('say a server sentence in the interface language, or nothing for a code they do not know', async () => {
    await setLanguage('de');
    expect(serverText('noUserNamed', { name: 'jdp' })).toBe('Den Benutzer jdp gibt es nicht');
    expect(serverText('itemsFailed', { n: 1, total: 3 })).toBe('1 von 3 Objekten ging nicht');
    expect(serverText('somethingNew')).toBeUndefined();
    expect(serverText(undefined)).toBeUndefined();
  });

  it('take the first browser language the app has, and English without one', () => {
    expect(browserLanguage(['nb-NO', 'de'])).toBe('no');
    expect(browserLanguage(['pt-BR'])).toBe('pt');
    expect(browserLanguage(['tlh', 'de-AT'])).toBe('de');
    expect(browserLanguage(['tlh'])).toBe('en');
  });

  it('use the browser language until one is picked', async () => {
    await setLanguage('');
    expect(language()).toBe(browserLanguage());
  });

  it('sort names by the letters of the language', async () => {
    const names = ['Zebra', 'Äpfel', 'Ost', 'Öl', 'datei10', 'datei9'];
    await setLanguage('de');
    expect([...names].sort(byName)).toEqual(['Äpfel', 'datei9', 'datei10', 'Öl', 'Ost', 'Zebra']);
    await setLanguage('sv');
    expect([...names].sort(byName)).toEqual(['datei9', 'datei10', 'Ost', 'Zebra', 'Äpfel', 'Öl']);
  });
});

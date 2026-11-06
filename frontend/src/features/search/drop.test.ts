import { afterAll, describe, expect, it } from 'vitest';
import { serverQuery } from '@/files/fields';
import { addMessages, setLanguage } from '@/i18n';
import de from './de.json';
import en from './en.json';
import { examples } from './drop';

addMessages('search', { de, en });
afterAll(() => setLanguage('de'));

describe('the examples of the search field', () => {
  it('are written in the words of the language', async () => {
    await setLanguage('de');
    expect(examples().map(([q]) => q)).toEqual(['*.mkv', '#steuer', 'bericht', 'IMG_????.jpg', 'breite:>=3840 codec:hevc', 'geändert:<7d']);
    await setLanguage('en');
    expect(examples().map(([q]) => q)).toEqual(['*.mkv', '#tax', 'report', 'IMG_????.jpg', 'width:>=3840 codec:hevc', 'modified:<7d']);
  });

  it('name fields the search knows', async () => {
    for (const lang of ['de', 'en', 'fr']) {
      await setLanguage(lang);
      const [, , , , meta, week] = examples().map(([q]) => serverQuery(q));
      expect([meta, week]).toEqual(['width:>=3840 vcodec:hevc', 'modified:<7d']);
    }
  });
});

import { describe, expect, it } from 'vitest';
import { addMessages, setLanguage } from '@/i18n';
import { plural } from '@/ui/kit';
import de from './de.json';
import en from './en.json';

addMessages('upload', { de, en });

describe('upload texts', () => {
  it('speak of one file in the singular', () => {
    setLanguage('de');
    expect(plural(1, 'upload.failed')).toMatch(/^Eine Datei ließ sich/);
    expect(plural(3, 'upload.failed')).toMatch(/^3 Dateien ließen sich/);
    setLanguage('en');
    expect(plural(1, 'upload.failed')).toMatch(/^One file could not/);
    expect(plural(3, 'upload.failed')).toMatch(/^3 files could not/);
  });
});

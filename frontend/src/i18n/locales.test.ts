import { describe, expect, it } from 'vitest';
import { compare, DRAFT, merge, rulesFor, STATUS, type Table } from './check';
import { BUNDLED, LANGUAGES } from './languages';
import { FIELD_IDS, fold, WORDS } from '@/files/fields';

const german = merge(import.meta.glob<Table>('/src/**/de.json', { eager: true, import: 'default' }));
const english = merge(import.meta.glob<Table>('/src/**/en.json', { eager: true, import: 'default' }));
const files = import.meta.glob<Table>('./locales/*.json', { eager: true, import: 'default' });
const fileOf = (code: string) => files[`./locales/${code}.json`];

const said = (table: Table, key: string) => {
  const v = key.split('.').reduce<Table | string | undefined>((at, k) => (typeof at === 'object' ? at[k] : undefined), table);
  return typeof v === 'string' ? fold(v) : '';
};
// A word the search line takes has to point at one field, and within a field at one value.
function clashes(table: Table) {
  const found: string[] = [];
  const unique = (what: string, pairs: [string, string][]) => {
    const seen = new Map<string, string>();
    for (const [id, word] of pairs) {
      if (!word) continue;
      if (seen.has(word)) found.push(`${what}: ${seen.get(word)} and ${id} are both "${word}"`);
      else seen.set(word, id);
    }
  };
  unique('fields', FIELD_IDS.map((id) => [id, said(table, `searchWords.${id}`) || said(table, `columns.${id}`)]));
  for (const [field, keys] of Object.entries(WORDS)) unique(field, Object.entries(keys).map(([code, key]) => [code, said(table, key)]));
  return found;
}

const report = (table: Table, code: string) => compare(german, table, rulesFor(LANGUAGES.find((l) => l.code === code)?.locale ?? code)).map((f) => `${f.key}: ${f.problem}`);

describe('the language files', () => {
  it('are there for every language but German and English, and for nothing else', () => {
    const want = LANGUAGES.map((l) => l.code).filter((c) => !BUNDLED.includes(c)).sort();
    expect(Object.keys(files).map((f) => f.match(/([\w-]+)\.json$/)![1]).sort()).toEqual(want);
  });

  it('English has German\'s keys, placeholders and plural forms', () => {
    expect(report(english, 'en')).toEqual([]);
  });

  it('give the Help sentence in GlimStone\'s words, the same in every app', () => {
    const help = (table: Table) => ((table.config as Table).info as Table).helpText;
    expect(help(english)).toBe('Problems, wishes or suggestions? You\'re welcome to open an issue on GitHub, or come say hi on Discord.');
    expect(help(german)).toBe('Probleme, Wünsche oder Verbesserungsvorschläge? Mach gern ein Issue auf GitHub auf, oder schau auf Discord vorbei.');
  });

  it('name GitHub and Discord in the Help sentence, the two places its buttons lead, and no mail', () => {
    const tables: [string, Table][] = [['de', german], ['en', english], ...Object.entries(files).map(([f, t]): [string, Table] => [f, t])];
    for (const [code, table] of tables) {
      const text = String(((table.config as Table).info as Table).helpText);
      expect(text, code).toMatch(/GitHub/);
      expect(text, code).toMatch(/Discord/);
      expect(text, code).not.toMatch(/e-?mail|e-?post|correo|почт|имейл|ایمیل|מייל|メール|이메일|邮件|อีเมล|ईमेल|البريد/i);
    }
  });

  it('German and English give each search word one meaning', () => {
    expect([...clashes(german), ...clashes(english)]).toEqual([]);
  });

  for (const l of LANGUAGES.filter((x) => !BUNDLED.includes(x.code))) {
    const table = fileOf(l.code) ?? {};
    const draft = table[STATUS] === DRAFT;
    it(`${l.code} has German's keys, placeholders and plural forms${draft ? ', not translated yet' : ''}`, () => {
      expect(report(table, l.code)).toEqual([]);
    });
    it(`${l.code} gives each search word one meaning`, () => {
      expect(clashes(table)).toEqual([]);
    });
  }
});

describe('the check', () => {
  const source: Table = { a: 'Kopiere {name}', n: { one: 'eine Datei', other: '{n} Dateien' } };
  const ru = rulesFor('ru');

  it('finds a missing key, an extra key and a wrong placeholder', () => {
    const found = compare(source, { a: 'Копировать {nam}', b: 'лишний', n: { one: '{n} файл', few: '{n} файла', many: '{n} файлов', other: '{n} файла' } }, ru);
    expect(found.map((f) => f.key)).toEqual(['a', 'b']);
    expect(compare(source, { n: { one: '{n} файл', few: '{n} файла', many: '{n} файлов', other: '{n} файла' } }, ru)).toEqual([{ key: 'a', problem: 'missing' }]);
  });

  it('asks for the plural forms of the language, and for the number where a form stands for more than one', () => {
    const found = compare(source, { a: 'Копировать {name}', n: { one: 'один файл', other: '{n} файла' } }, ru).map((f) => f.key);
    expect(found).toEqual(['n.one', 'n.few', 'n.many']);
    expect(compare(source, { a: 'Kopieer {name}', n: { one: 'één bestand', other: '{n} bestanden' } }, rulesFor('nl'))).toEqual([]);
  });

  it('lets a file leave texts empty only while it says it is not translated yet', () => {
    const empty = { a: '', n: { one: '', other: '' } };
    expect(compare(source, empty, rulesFor('nl')).map((f) => f.problem)).toEqual(['empty', 'empty', 'empty']);
    expect(compare(source, { [STATUS]: DRAFT, ...empty }, rulesFor('nl'))).toEqual([]);
  });
});

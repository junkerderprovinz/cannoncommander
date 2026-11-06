// The server knows the fields of the search line by their ids and German names. People type them in their own
// language, so a column's name goes to the server as its id and a word the column shows as the code behind it.
import de from '@/i18n/de.json';
import en from '@/i18n/en.json';
import { language, LANGUAGES, t } from '@/i18n';
import { LANGUAGE_COLUMNS, languageName, META_WORDS } from './metaText';

type Words = Record<string, Record<string, string>>;
const COLUMN_IDS = Object.keys(de.columns);
const TABLES = [en, de].map((tb) => ({ columns: tb.columns as Record<string, string>, words: tb.metaValues as unknown as Words }));

// No case, no accents and ß as ss, as the server compares field names, in any script.
const fold = (s: string) => s.toLowerCase().replace(/ß/g, 'ss').normalize('NFD').replace(/[^\p{L}\p{N}]/gu, '');

interface Lookup { fields: Map<string, string>; values: Map<string, Map<string, string>> }
let built: { lang: string; lookup: Lookup } | undefined;

function lookup(): Lookup {
  if (built?.lang === language()) return built.lookup;
  const fields = new Map<string, string>();
  const add = <K, V>(m: Map<K, V>, k: K, v: V) => { if (!m.has(k)) m.set(k, v); };
  for (const id of COLUMN_IDS) add(fields, fold(t(`columns.${id}`)), id);
  for (const tb of TABLES) for (const id of COLUMN_IDS) add(fields, fold(tb.columns[id] ?? ''), id);
  fields.delete('');
  const values = new Map<string, Map<string, string>>();
  for (const [id, codes] of Object.entries(META_WORDS)) {
    const m = new Map<string, string>();
    for (const code of codes) {
      add(m, fold(t(`metaValues.${id}.${code}`)), code);
      for (const tb of TABLES) add(m, fold(tb.words[id]?.[code] ?? ''), code);
    }
    values.set(id, m);
  }
  const names = new Map<string, string>();
  for (const lang of [language(), 'en', 'de']) {
    for (const code of [...LANGUAGES.map((l) => l.code), 'und']) add(names, fold(languageName(code, lang)), code);
  }
  for (const id of LANGUAGE_COLUMNS) values.set(id, names);
  built = { lang: language(), lookup: { fields, values } };
  return built.lookup;
}

const token = /([\p{L}][\p{L}\d.-]*):(>=|<=|!=|>|<|=)?("[^"]*"?|\S*)|"[^"]*"?|\S+/gu;

export function serverQuery(line: string): string {
  const { fields, values } = lookup();
  return line.replace(token, (all, name: string | undefined, op = '', value = '') => {
    if (!name) return all;
    const id = fields.get(fold(name));
    if (!id) return all;
    const code = values.get(id)?.get(fold(value.replace(/^"|"$/g, '')));
    return `${id}:${op}${code ?? value}`;
  });
}

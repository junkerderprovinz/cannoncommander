// The server knows the fields of the search line by their ids, and their values as codes, ISO dates and numbers with a
// point. People type them in their own language, so a line goes out in the server's form and a saved line comes back
// in the language the interface shows.
import de from '@/i18n/de.json';
import en from '@/i18n/en.json';
import { language, LANGUAGES, locale, t } from '@/i18n';
import { LANGUAGE_COLUMNS, languageName, META_WORDS } from './metaText';

type Table = { [key: string]: string | Table };
const FIELD_IDS = [...Object.keys(de.columns), ...Object.keys(de.searchWords)];

// German words the server also takes for its fields.
const ALIASES: Record<string, string> = {
  kamera: 'camera', interpret: 'artist', kunstler: 'artist', kuenstler: 'artist', datum: 'modified', geaendert: 'modified',
  lange: 'duration', laenge: 'duration', groesse: 'size', hoehe: 'height',
};

// The codes of the columns that show words, each with the key of its word.
const codes = (keys: string[], key: (code: string) => string) => Object.fromEntries(keys.map((c) => [c, key(c)]));
const WORDS: Record<string, Record<string, string>> = {
  type: { folder: 'kinds.folder', link: 'kinds.link', ...codes(Object.keys(de.types), (c) => `types.${c}`) },
  kind: codes(['folder', 'image', 'video', 'music', 'archive', 'disk', 'document', 'text', 'code', 'other'], (c) => `groups.${c}`),
  ...Object.fromEntries(Object.entries(META_WORDS).map(([id, list]) => [id, codes(list, (c) => `metaValues.${id}.${c}`)])),
  ...Object.fromEntries(['encrypted', 'archcrypt', 'bootable', 'signed'].map((id) => [id, { yes: 'files.yes', no: 'files.no' }])),
};
const DATES = new Set(['modified', 'created', 'accessed', 'statchange', 'taken']);
const SIZES = new Set(['size', 'disksize', 'unpacked']);
const NUMBERS = new Set(['width', 'height', 'duration', 'year', 'season', 'episode', 'track', 'iso', 'pages', 'words', 'lines',
  'entries', 'items', 'hardlinks', 'inode', 'blocks', 'uid', 'gid', 'vbitrate']);
const COMPARE = new Set(['<', '>', '<=', '>=']);
const UNITS: [string, string][] = [['byte', 'B'], ['kilobyte', 'KB'], ['megabyte', 'MB'], ['gigabyte', 'GB'], ['terabyte', 'TB']];
const DAY = 86400000;

// No case, no accents and ß as ss, as the server compares field names, in any script.
const fold = (s: string) => s.toLowerCase().replace(/ß/g, 'ss').normalize('NFD').replace(/[^\p{L}\p{N}]/gu, '');
const at = (table: Table, key: string) => key.split('.').reduce<Table | string | undefined>((v, k) => (typeof v === 'object' ? v[k] : undefined), table);
// A key's text in the interface's language, in English and in German, the order in which they count.
const SOURCES = [(key: string) => t(key), (key: string) => at(en, key), (key: string) => at(de, key)];
const text = (source: (key: string) => unknown, key: string) => {
  const s = source(key);
  return typeof s === 'string' && s !== key ? s : '';
};

// Digits as the language writes them, and as a Japanese or Chinese keyboard types them.
function ascii(s: string, loc: string) {
  const own = new Intl.NumberFormat(loc, { useGrouping: false }).format(9876543210);
  return s.replace(/\p{Nd}/gu, (c) => {
    const i = own.indexOf(c);
    if (i >= 0) return String(9 - i);
    const wide = '０１２３４５６７８９'.indexOf(c);
    return wide >= 0 ? String(wide) : c;
  });
}

const dateFormat = (loc: string) => new Intl.DateTimeFormat(loc, { day: '2-digit', month: '2-digit', year: 'numeric', timeZone: 'UTC' });

interface Lookup {
  loc: string;
  decimal: string;
  fields: Map<string, string>;
  names: Map<string, string>;
  values: Map<string, Map<string, string>>;
  units: Map<string, string>;
}
let built: { lang: string; lookup: Lookup } | undefined;

function lookup(): Lookup {
  if (built?.lang === language()) return built.lookup;
  const loc = locale();
  const add = <K, V>(m: Map<K, V>, k: K, v: V) => { if (k !== '' && !m.has(k)) m.set(k, v); };
  const fields = new Map<string, string>();
  for (const id of FIELD_IDS) add(fields, fold(id), id);
  for (const source of SOURCES) {
    for (const id of FIELD_IDS) {
      add(fields, fold(text(source, `searchWords.${id}`)), id);
      add(fields, fold(text(source, `columns.${id}`)), id);
    }
  }
  for (const [word, id] of Object.entries(ALIASES)) add(fields, word, id);

  const names = new Map<string, string>();
  for (const id of FIELD_IDS) {
    const name = text(t, `searchWords.${id}`) || text(t, `columns.${id}`);
    if (name) names.set(id, name.toLocaleLowerCase(loc).replace(/[^\p{L}\p{M}\p{N}]/gu, ''));
  }

  const values = new Map<string, Map<string, string>>();
  for (const [id, keys] of Object.entries(WORDS)) {
    const m = new Map<string, string>();
    for (const code of Object.keys(keys)) add(m, fold(code), code);
    for (const source of SOURCES) for (const [code, key] of Object.entries(keys)) add(m, fold(text(source, key)), code);
    values.set(id, m);
  }
  const langs = new Map<string, string>();
  const langCodes = [...LANGUAGES.map((l) => l.code), 'und'];
  for (const code of langCodes) add(langs, fold(code), code);
  for (const lang of [language(), 'en', 'de']) for (const code of langCodes) add(langs, fold(languageName(code, lang)), code);
  for (const id of LANGUAGE_COLUMNS) values.set(id, langs);

  const units = new Map<string, string>(Object.entries({ b: 'B', k: 'KB', kb: 'KB', m: 'MB', mb: 'MB', g: 'GB', gb: 'GB', t: 'TB', tb: 'TB' }));
  for (const l of [loc, 'en', 'de']) {
    for (const [unit, code] of UNITS) {
      add(units, fold(new Intl.NumberFormat(l, { style: 'unit', unit }).formatToParts(1).find((p) => p.type === 'unit')?.value ?? ''), code);
    }
  }
  const decimal = new Intl.NumberFormat(loc).formatToParts(1.5).find((p) => p.type === 'decimal')?.value ?? '.';
  built = { lang: language(), lookup: { loc, decimal, fields, names, values, units } };
  return built.lookup;
}

// One separator is the decimal one, a separator that comes more than once groups thousands, and with both kinds the
// last one is the decimal one. So 2,5 and 2.5 are the same in every language, and a line read twice stays as it was.
function plainNumber(s: string): string | undefined {
  const m = /^([+-]?)(\d+(?:[.,]\d+)*)$/.exec(s.replace(/٫/g, '.').replace(/٬/g, ','));
  if (!m) return undefined;
  const body = m[2]!, seps = body.match(/[.,]/g) ?? [];
  const cut = new Set(seps).size === 2 || seps.length === 1 ? Math.max(body.lastIndexOf('.'), body.lastIndexOf(',')) : -1;
  const whole = (cut < 0 ? body : body.slice(0, cut)).replace(/[.,]/g, '');
  return m[1] + String(Number(`${whole}.${cut < 0 ? 0 : body.slice(cut + 1)}`));
}

const parts = (v: string, lk: Lookup) => /^([+-]?[\d.,٫٬]+)\s*(\p{L}[\p{L}.]*)?$/u.exec(ascii(v.trim(), lk.loc));

function number(v: string, lk: Lookup) {
  const m = parts(v, lk), n = m && plainNumber(m[1]!);
  return n === undefined || !m ? undefined : n + (m[2] ?? '');
}

function size(v: string, lk: Lookup) {
  const m = parts(v, lk), n = m && plainNumber(m[1]!);
  const unit = m?.[2] ? lk.units.get(fold(m[2])) : '';
  return n === undefined || !m || unit === undefined ? undefined : n + unit;
}

// The day a date names in the calendar of loc, found near a guess, since a language such as Thai or Persian counts
// its years and months in a calendar of its own.
function dayOf(nums: number[], loc: string): string | undefined {
  const fmt = dateFormat(loc);
  const order = fmt.formatToParts(0).map((p) => p.type).filter((p) => p === 'day' || p === 'month' || p === 'year');
  const want = Object.fromEntries(order.map((p, i) => [p, nums[i]!])) as Record<'day' | 'month' | 'year', number>;
  if (order.length !== 3 || want.year < 1000 || want.month > 12 || want.day > 31) return undefined;
  const of = (ms: number) => {
    const p = Object.fromEntries(fmt.formatToParts(ms).map((x) => [x.type, Number(ascii(x.value, loc))]));
    return [p.year, p.month, p.day];
  };
  const ref = Date.UTC(2000, 0, 1), [ry, rm, rd] = of(ref);
  const guess = ref + Math.round((want.year - ry!) * 365.2425 + (want.month - rm!) * 30.44 + (want.day - rd!)) * DAY;
  for (let k = 0; k <= 45; k++) {
    for (const ms of [guess + k * DAY, guess - k * DAY]) {
      const [y, mo, d] = of(ms);
      if (y === want.year && mo === want.month && d === want.day) return new Date(ms).toISOString().slice(0, 10);
    }
  }
  return undefined;
}

function isoDate(v: string, lk: Lookup) {
  if (/^\d{4}-\d{1,2}-\d{1,2}$/.test(v)) return undefined;
  const m = /^\D*(\d{1,4})\D+(\d{1,4})\D+(\d{1,4})\D*$/.exec(ascii(v, lk.loc));
  if (!m) return undefined;
  const nums = m.slice(1).map(Number);
  for (const loc of [lk.loc, 'de-AT', 'en-GB']) {
    const day = dayOf(nums, loc);
    if (day) return day;
  }
  return undefined;
}

const unquote = (v: string) => v.replace(/^"|"$/g, '');
const asNumber = (id: string, op: string) => NUMBERS.has(id) || COMPARE.has(op);

function serverValue(lk: Lookup, id: string, op: string, raw: string) {
  const v = unquote(raw);
  const code = lk.values.get(id)?.get(fold(v));
  if (code) return code;
  const out = DATES.has(id) ? isoDate(v, lk) : SIZES.has(id) ? size(v, lk) : asNumber(id, op) ? number(v, lk) : undefined;
  return out ?? raw;
}

function localValue(lk: Lookup, id: string, op: string, v: string) {
  if (v.startsWith('"')) return v;
  const key = WORDS[id]?.[v];
  let out = key ? t(key) : LANGUAGE_COLUMNS.includes(id) && lk.values.get(id)?.get(fold(v)) === v ? languageName(v) : v;
  const iso = DATES.has(id) && /^(\d{4})-(\d{2})-(\d{2})$/.exec(v);
  if (iso) out = dateFormat(lk.loc).format(Date.UTC(+iso[1]!, +iso[2]! - 1, +iso[3]!)).replace(/[\s‎‏؜]/g, '');
  else if (SIZES.has(id) || asNumber(id, op)) out = out.replace(/^([+-]?\d+)\.(\d)/, `$1${lk.decimal}$2`);
  return /\s/.test(out) ? `"${out}"` : out;
}

const token = /([\p{L}][\p{L}\p{M}\p{N}.-]*):(>=|<=|!=|>|<|=)?("[^"]*"?|\S*)|"[^"]*"?|\S+/gu;

function fieldsOf(line: string, each: (lk: Lookup, id: string, op: string, value: string) => string) {
  const lk = lookup();
  return line.replace(token, (all, name: string | undefined, op = '', raw = '') => {
    const id = name && lk.fields.get(fold(name));
    return id ? each(lk, id, op, serverValue(lk, id, op, raw)) : all;
  });
}

// A line as the server reads it. Names the interface does not know stay for the server to name.
export const serverQuery = (line: string) => fieldsOf(line, (_lk, id, op, value) => `${id}:${op}${value}`);

// A line in the words, dates and numbers of the interface's language.
export const localQuery = (line: string) => fieldsOf(line, (lk, id, op, value) => `${lk.names.get(id) ?? id}:${op}${localValue(lk, id, op, value)}`);

// A size of the search window as the server reads it, such as 2.5GB for 2,5 GB.
export const serverSize = (v: string) => (v.trim() ? size(v, lookup()) ?? v : v);

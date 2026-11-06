// What a language file has to match: German's keys, each sentence's placeholders, and in a counted text the plural
// forms its own language uses. The parity test and the translators' script both run it, so it imports nothing.
export type Table = { [key: string]: string | Table };

// A language file still being translated says so at its top. Only then may a text be left empty.
export const STATUS = '$status';
export const DRAFT = 'untranslated';

// The table a de.json or en.json file fills: the global texts at the top, everything else under its folder's name.
export function namespaceOf(file: string): string {
  const path = file.replace(/\\/g, '/');
  const feature = path.match(/features\/([^/]+)\/(?:de|en)\.json$/);
  if (feature) return feature[1]!;
  if (/i18n\/(?:de|en)\.json$/.test(path)) return '';
  if (/share\/(?:de|en)\.json$/.test(path)) return 'visit';
  return path.match(/([^/]+)\/(?:de|en)\.json$/)![1]!;
}

export function merge(files: Record<string, Table>): Table {
  const all: Table = {};
  for (const [file, table] of Object.entries(files)) {
    const ns = namespaceOf(file);
    if (ns) all[ns] = table;
    else Object.assign(all, table);
  }
  return all;
}

const isForms = (v: string | Table | undefined): v is Table =>
  typeof v === 'object' && Object.keys(v).length === 2 && 'one' in v && 'other' in v;
const holes = (text: string) => [...new Set(text.match(/\{\w+\}/g) ?? [])].sort();
const same = (a: string[], b: string[]) => a.length === b.length && a.every((x, i) => x === b[i]);

export interface Rules {
  categories: string[];
  // Categories that only ever stand for one number, such as one in German, may write that number out instead of {n}.
  single: Set<string>;
}

export function rulesFor(locale: string): Rules {
  const rules = new Intl.PluralRules(locale);
  const categories = rules.resolvedOptions().pluralCategories as string[];
  const seen = new Map<string, number>();
  for (let n = 0; n <= 1000; n++) {
    const c = rules.select(n);
    seen.set(c, (seen.get(c) ?? 0) + 1);
  }
  return { categories, single: new Set(categories.filter((c) => seen.get(c) === 1)) };
}

export interface Finding { key: string; problem: string }

export function compare(source: Table, target: Table, rules: Rules): Finding[] {
  const draft = target[STATUS] === DRAFT;
  const found: Finding[] = [];
  const say = (key: string, problem: string) => found.push({ key, problem });

  const sentence = (key: string, want: string[], got: unknown, optional: string[] = []) => {
    if (typeof got !== 'string') {
      say(key, got === undefined ? 'missing' : 'should be a text');
      return;
    }
    if (!got) {
      if (!draft) say(key, 'empty');
      return;
    }
    const have = holes(got);
    const lacking = want.filter((h) => !have.includes(h) && !optional.includes(h));
    const strange = have.filter((h) => !want.includes(h));
    if (lacking.length || strange.length) say(key, `placeholders: ${[...lacking.map((h) => 'lacks ' + h), ...strange.map((h) => 'has unknown ' + h)].join(', ')}`);
  };

  const walk = (src: Table, tgt: Table | undefined, at: string) => {
    for (const [name, value] of Object.entries(src)) {
      const key = at + name;
      const got = tgt?.[name];
      if (typeof value === 'string') {
        sentence(key, holes(value), got);
      } else if (isForms(value)) {
        if (typeof got !== 'object') { say(key, got === undefined ? 'missing' : 'should hold the plural forms'); continue; }
        const want = holes(value.other as string);
        for (const c of rules.categories) sentence(`${key}.${c}`, want, got[c], rules.single.has(c) ? ['{n}'] : []);
        for (const c of Object.keys(got)) if (!rules.categories.includes(c)) say(`${key}.${c}`, 'is not a plural form of this language');
      } else {
        if (typeof got !== 'object') { say(key, got === undefined ? 'missing' : 'should be a group'); continue; }
        walk(value, got, key + '.');
      }
    }
    for (const name of Object.keys(tgt ?? {})) {
      if (!(name in src) && !(at === '' && name.startsWith('$'))) say(at + name, 'is not in German');
    }
  };
  walk(source, target, '');
  return found;
}

// A language file with every key of German, its own plural forms, and whatever it had translated already.
export function skeleton(source: Table, old: Table, rules: Rules): Table {
  const fill = (src: Table, had: Table | undefined): Table => {
    const out: Table = {};
    for (const [name, value] of Object.entries(src)) {
      const prev = had?.[name];
      if (typeof value === 'string') out[name] = typeof prev === 'string' ? prev : '';
      else if (isForms(value)) out[name] = Object.fromEntries(rules.categories.map((c) => [c, typeof prev === 'object' && typeof prev[c] === 'string' ? prev[c] : '']));
      else out[name] = fill(value, typeof prev === 'object' ? prev : undefined);
    }
    return out;
  };
  const body = fill(source, old);
  const untranslated = (t: Table): boolean => Object.values(t).some((v) => (typeof v === 'string' ? !v : untranslated(v)));
  return untranslated(body) ? { [STATUS]: DRAFT, ...body } : body;
}

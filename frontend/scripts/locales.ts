// node scripts/locales.ts sync [code ...]   writes every language file anew from German, keeping what is translated
// node scripts/locales.ts check [code ...]  lists what a language file lacks or has wrong, and fails if anything does
// node scripts/locales.ts todo code         prints every text still open in that file, beside German and English
// Without codes both work on every language. A translator runs check on their own file until it says nothing.
import { existsSync, readdirSync, readFileSync, statSync, writeFileSync } from 'node:fs';
import { join, relative, resolve } from 'node:path';
import { compare, DRAFT, merge, rulesFor, skeleton, STATUS, type Table } from '../src/i18n/check.ts';
import { BUNDLED, LANGUAGES } from '../src/i18n/languages.ts';

const src = resolve(import.meta.dirname, '..', 'src');
const dir = join(src, 'i18n', 'locales');

function sources(at: string, out: Record<string, Table> = {}) {
  for (const name of readdirSync(at)) {
    const path = join(at, name);
    if (statSync(path).isDirectory()) sources(path, out);
    else if (name === 'de.json') out[relative(src, path)] = JSON.parse(readFileSync(path, 'utf8'));
  }
  return out;
}

const german = merge(sources(src));
const english = merge(Object.fromEntries(Object.entries(sources(src)).map(([file]) => [file, JSON.parse(readFileSync(join(src, file.replace(/de\.json$/, 'en.json')), 'utf8'))])));
const [command, ...asked] = process.argv.slice(2);
const languages = LANGUAGES.filter((l) => !BUNDLED.includes(l.code) && (!asked.length || asked.includes(l.code)));
const unknown = asked.filter((c) => !languages.some((l) => l.code === c));
if (unknown.length) {
  console.error(`not a language of the app: ${unknown.join(', ')}`);
  process.exit(2);
}
const fileOf = (code: string) => join(dir, code + '.json');
const read = (code: string): Table => (existsSync(fileOf(code)) ? JSON.parse(readFileSync(fileOf(code), 'utf8')) : {});

if (command === 'sync') {
  for (const l of languages) {
    writeFileSync(fileOf(l.code), JSON.stringify(skeleton(german, read(l.code), rulesFor(l.locale ?? l.code)), null, 2) + '\n');
  }
  console.log(`wrote ${languages.length} language files to ${relative(process.cwd(), dir)}`);
} else if (command === 'check') {
  let bad = 0;
  for (const l of languages) {
    const table = read(l.code);
    const found = compare(german, table, rulesFor(l.locale ?? l.code));
    const empty = JSON.stringify(table).match(/":""/g)?.length ?? 0;
    const state = table[STATUS] === DRAFT ? `not translated yet, ${empty} texts open` : 'translated';
    console.log(`${l.code} ${l.name}: ${found.length ? `${found.length} problems` : 'in order'}, ${state}`);
    for (const f of found) console.log(`  ${f.key}: ${f.problem}`);
    bad += found.length;
  }
  process.exit(bad ? 1 : 0);
} else if (command === 'todo' && languages.length === 1) {
  const at = (t: Table | string | undefined, key: string[]) => key.reduce<Table | string | undefined>((v, k) => (typeof v === 'object' ? v[k] : undefined), t);
  // A plural form German does not have shows German's other form, the one it is built from.
  const shown = (t: Table, key: string[]) => {
    const v = at(t, key) ?? at(t, [...key.slice(0, -1), 'other']);
    return typeof v === 'string' ? v : '';
  };
  const open = (t: Table, path: string[]) => {
    for (const [k, v] of Object.entries(t)) {
      if (k.startsWith('$')) continue;
      if (typeof v === 'object') open(v, [...path, k]);
      else if (!v) console.log([[...path, k].join('.'), shown(german, [...path, k]), shown(english, [...path, k])].join('\t'));
    }
  };
  open(read(languages[0]!.code), []);
} else {
  console.error('usage: node scripts/locales.ts sync|check [code ...] or todo code');
  process.exit(2);
}

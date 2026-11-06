// Texts live in de.json and en.json, the global ones here and each feature's beside its code under the feature's name.
// Every other language is one file under locales/ that holds all of them, loaded only once someone picks it.
import { reactive } from 'vue';
import de from './de.json';
import en from './en.json';
import { LANGUAGES, type LanguageInfo, PSEUDO } from './languages';
import { pseudo } from './pseudo';

export type Table = { [key: string]: string | Table };
export type Loader = () => Promise<Table>;
export { LANGUAGES, type LanguageInfo };

const tables: Record<string, Table> = { de: structuredClone(de), en: structuredClone(en) };
const state = reactive({ lang: 'de' });
let loaders: Record<string, Loader> = {};
let asked = 0;

// The pseudo languages are offered by the development server only, so a build never shows them.
export const languages = (): LanguageInfo[] => (import.meta.env.DEV ? [...LANGUAGES, ...PSEUDO] : LANGUAGES);
const infoOf = (code: string) => languages().find((l) => l.code === code);
const isPseudo = (code: string) => import.meta.env.DEV && PSEUDO.some((l) => l.code === code);

// Each page hands over how it loads a language file, as the share page needs far fewer texts than the app.
export function useLocales(files: Record<string, () => Promise<unknown>>) {
  loaders = Object.fromEntries(Object.entries(files).map(([path, load]) => [path.match(/([\w-]+)\.json/)![1]!, load as Loader]));
}

// The first language of the browser that the app has, matched on the language alone, and English when there is none.
export function browserLanguage(wanted: readonly string[] = typeof navigator === 'undefined' ? [] : navigator.languages ?? [navigator.language]): string {
  const alias: Record<string, string> = { nb: 'no', nn: 'no' };
  for (const tag of wanted) {
    const base = tag.toLowerCase().split('-')[0]!;
    const code = alias[base] ?? base;
    if (LANGUAGES.some((l) => l.code === code)) return code;
  }
  return 'en';
}

export async function setLanguage(code: string) {
  const lang = code && infoOf(code) ? code : browserLanguage();
  const mine = ++asked;
  if (!tables[lang] && loaders[lang]) {
    const table = await loaders[lang]!().catch(() => ({}));
    tables[lang] = table;
  }
  if (mine !== asked) return;
  state.lang = lang;
  if (typeof document !== 'undefined') {
    document.documentElement.lang = lang;
    document.documentElement.dir = infoOf(lang)?.rtl ? 'rtl' : 'ltr';
  }
}
export const language = () => state.lang;
export const locale = () => infoOf(state.lang)?.locale ?? state.lang;

export function addMessages(namespace: string, texts: Partial<Record<'de' | 'en', Table>>) {
  for (const lang of ['de', 'en'] as const) {
    if (texts[lang]) tables[lang]![namespace] = texts[lang]!;
  }
}

const lookup = (table: Table | undefined, key: string) => {
  let at: string | Table | undefined = table;
  for (const part of key.split('.')) at = typeof at === 'object' ? at[part] : undefined;
  return at;
};
// A text a language file leaves empty is not translated yet, and the next language in line speaks for it.
const said = (at: string | Table | undefined) => (typeof at === 'string' && at ? at : undefined);

// German is the source and English the second language, so a gap anywhere falls to English and then to German.
const chain = (lang: string) => (lang === 'de' ? ['de'] : lang === 'en' ? ['en', 'de'] : [lang, 'en', 'de']);

const fill = (text: string, params: Record<string, string | number>) =>
  text.replace(/\{(\w+)\}/g, (all, name: string) => (name in params ? String(params[name]) : all));
const shown = (text: string) => (isPseudo(state.lang) ? pseudo(text, state.lang) : text);

// A missing key shows itself, so a gap is easy to spot.
export function t(key: string, params: Record<string, string | number> = {}): string {
  for (const lang of chain(state.lang)) {
    const text = said(lookup(tables[lang], key));
    if (text) return fill(shown(text), params);
  }
  return key;
}

// For labels that must follow a change of language without being registered again.
export const tl = (key: string, params?: Record<string, string | number>) => () => t(key, params);

const rules = new Map<string, Intl.PluralRules>();
const ruleOf = (lang: string) => {
  if (!rules.has(lang)) rules.set(lang, new Intl.PluralRules(infoOf(lang)?.locale ?? lang));
  return rules.get(lang)!;
};

export const num = (n: number) => n.toLocaleString(locale());

// Names sort the way the language orders its letters, and numbers inside a name by their value.
const collators = new Map<string, Intl.Collator>();
const collator = (numeric: boolean) => {
  const key = locale() + numeric;
  if (!collators.has(key)) collators.set(key, new Intl.Collator(locale(), { numeric }));
  return collators.get(key)!;
};
export const byName = (a: string, b: string) => collator(true).compare(a, b);
export const byText = (a: string, b: string) => collator(false).compare(a, b);

// What the server said as a code with parameters, in the interface's language. A code this interface does not know
// gives nothing, and the caller shows the server's own sentence instead.
export function serverText(code?: string, params: Record<string, unknown> = {}): string | undefined {
  const key = 'server.' + code;
  if (!code || !chain(state.lang).some((lang) => lookup(tables[lang], key) !== undefined)) return undefined;
  const { n, ...rest } = Object.fromEntries(Object.entries(params).map(([k, v]) => [k, typeof v === 'number' ? v : String(v)]));
  return typeof n === 'number' && typeof lookup(tables.de, key) === 'object' ? plural(n, key, rest) : t(key, n === undefined ? rest : { n, ...rest });
}

// key names the forms of a counted text, one and other in German, and in every language the forms its plural rules
// ask for. n reaches the text as {n}, written the way the language writes numbers.
export function plural(n: number, key: string, params: Record<string, string | number> = {}): string {
  for (const lang of chain(state.lang)) {
    const forms = lookup(tables[lang], key);
    if (typeof forms !== 'object') continue;
    const text = said(forms[ruleOf(lang).select(n)]) ?? said(forms.other);
    if (text) return fill(shown(text), { n: num(n), ...params });
  }
  return key;
}

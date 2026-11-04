// Texts live in de.json and en.json, the global ones here and each feature's beside its code under the feature's name.
import { reactive } from 'vue';
import de from './de.json';
import en from './en.json';

type Table = { [key: string]: string | Table };
export type Language = 'de' | 'en';

const tables: Record<Language, Table> = { de: structuredClone(de), en: structuredClone(en) };
const state = reactive({ lang: 'de' as Language });

export const LOCALES: Record<Language, string> = { de: 'de-AT', en: 'en-GB' };

export function setLanguage(lang: string) {
  state.lang = lang === 'en' ? 'en' : 'de';
  if (typeof document !== 'undefined') document.documentElement.lang = state.lang;
}
export const language = () => state.lang;
export const locale = () => LOCALES[state.lang];

export function addMessages(namespace: string, texts: Partial<Record<Language, Table>>) {
  for (const lang of Object.keys(tables) as Language[]) {
    if (texts[lang]) tables[lang][namespace] = texts[lang];
  }
}

const lookup = (table: Table, key: string) => {
  let at: string | Table | undefined = table;
  for (const part of key.split('.')) at = typeof at === 'object' ? at[part] : undefined;
  return typeof at === 'string' ? at : undefined;
};

// A key English does not have yet falls back to German, and a missing key shows itself, so a gap is easy to spot.
export function t(key: string, params: Record<string, string | number> = {}): string {
  const text = lookup(tables[state.lang], key) ?? lookup(tables.de, key) ?? key;
  return text.replace(/\{(\w+)\}/g, (all, name: string) => (name in params ? String(params[name]) : all));
}

// For labels that must follow a change of language without being registered again.
export const tl = (key: string, params?: Record<string, string | number>) => () => t(key, params);

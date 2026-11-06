// fs/meta keeps words as codes and numbers with a point. These turn them into the interface's language.
import { locale, plural, t } from '@/i18n';

// The codes each word column can hold. Anything else is shown as the server sent it, such as 5.1 or Zstandard.
export const META_WORDS: Record<string, string[]> = {
  orientation: ['landscape', 'portrait', 'square'],
  flash: ['fired', 'off'],
  channels: ['mono', 'stereo'],
  method: ['none'],
  arch: ['all'],
};
export const LANGUAGE_COLUMNS = ['audiotracks', 'subtitles'];

export function decimals(text: string) {
  const sep = new Intl.NumberFormat(locale()).formatToParts(1.5).find((p) => p.type === 'decimal')?.value ?? '.';
  return text.replace(/(\d)\.(\d)/g, (_all, a: string, b: string) => a + sep + b);
}

// A track's language by its code, in the interface's language. und is a track that does not say.
export function languageName(code: string, lang = locale()) {
  if (code === 'und') return t('metaValues.unknownLanguage');
  try {
    const name = new Intl.DisplayNames([lang], { type: 'language', fallback: 'none' }).of(code);
    if (name) return name;
  } catch {
    // A code Intl cannot parse is shown as it is.
  }
  return code.toUpperCase();
}

export function metaWord(id: string, value: string) {
  if (LANGUAGE_COLUMNS.includes(id)) return value.split(', ').map((c) => languageName(c)).join(', ');
  if (/^\d+$/.test(value)) {
    if (id === 'channels') return plural(+value, 'metaValues.channelCount');
    if (id === 'method') return t('metaValues.methodNumber', { n: value });
  }
  return META_WORDS[id]?.includes(value) ? t(`metaValues.${id}.${value}`) : value;
}

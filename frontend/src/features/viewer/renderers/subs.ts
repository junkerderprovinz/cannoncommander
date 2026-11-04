// Subtitles beside a film: which files belong to it, what language they are and their cues.
import { t } from '@/i18n';
import { ext, stem } from '../kinds';

export interface Cue { start: number; end: number; text: string }

const stamp = (s: string) => {
  const m = s.trim().match(/^(?:(\d+):)?(\d{1,2}):(\d{2})[,.](\d{1,3})$/);
  return m ? (+(m[1] || 0)) * 3600 + +m[2]! * 60 + +m[3]! + +m[4]!.padEnd(3, '0') / 1000 : NaN;
};

// SRT and WebVTT share the timing line, so one reader takes both.
export function parseCues(text: string): Cue[] {
  const cues: Cue[] = [];
  for (const block of text.replace(/\r/g, '').split(/\n{2,}/)) {
    const lines = block.split('\n');
    const at = lines.findIndex((l) => l.includes('-->'));
    if (at < 0) continue;
    const [a, b] = lines[at]!.split('-->');
    const start = stamp(a!), end = stamp(b!.trim().split(/\s+/)[0]!);
    if (Number.isNaN(start) || Number.isNaN(end)) continue;
    cues.push({ start, end, text: lines.slice(at + 1).join('\n').replace(/<[^>]+>/g, '').trim() });
  }
  return cues;
}

export const cueAt = (cues: Cue[], s: number) => cues.find((c) => s >= c.start && s <= c.end)?.text ?? '';

const SUB_EXT = ['srt', 'vtt'];
const LANGS = ['de', 'en', 'fr', 'es', 'it', 'nl', 'pl', 'cs', 'hu', 'tr', 'pt', 'ru', 'ja'];

export interface SubFile { name: string; label: string }

// Film.srt, Film.de.srt and Film.en.forced.vtt all belong to Film.mkv.
export function subsFor(video: string, names: string[]): SubFile[] {
  const base = stem(video).toLowerCase();
  return names
    .filter((n) => SUB_EXT.includes(ext(n)) && (stem(n).toLowerCase() === base || stem(n).toLowerCase().startsWith(base + '.')))
    .map((name) => {
      const tags = stem(name).slice(base.length).toLowerCase().split('.').filter(Boolean);
      const lang = tags.find((x) => LANGS.includes(x));
      return { name, label: lang ? t(`viewer.langs.${lang}`) : tags.length ? tags.join(', ') : t('viewer.media.subFile') };
    });
}

// Sizes, dates and durations the way the whole interface writes them.
import { language, locale, t } from '@/i18n';
import { prefs } from './prefs';

export const KB = 1024, MB = KB * 1024, GB = MB * 1024, TB = GB * 1024;
export const DAY = 86400000;

// The parity runs stop the clock, so relative dates read the same in the prototype and here.
let clock = () => Date.now();
export const now = () => clock();
export const setClock = (fn: () => number) => { clock = fn; };

export function fmtSize(b: number | null | undefined): string {
  if (b == null) return '';
  for (const [n, v] of [['TB', TB], ['GB', GB], ['MB', MB], ['KB', KB]] as const) {
    if (b >= v) return (b / v).toLocaleString(locale(), { maximumFractionDigits: b / v < 10 ? 1 : 0 }) + ' ' + n;
  }
  return b + ' B';
}

// Austria's own locale groups thousands with a space; the lists group them with a dot, as German text does.
export const fmtCount = (n: number) => n.toLocaleString(language() === 'de' ? 'de' : locale());

export const fmtDate = (ms: number) => new Date(ms).toLocaleDateString(locale(), { day: '2-digit', month: '2-digit', year: 'numeric' });
export const fmtTime = (ms: number) => new Date(ms).toLocaleString(locale(), { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' });

export function fmtAgo(ms: number): string {
  const days = Math.floor((now() - ms) / DAY);
  if (days <= 0) return t('format.today');
  if (days === 1) return t('format.yesterday');
  if (days < 30) return t('format.daysAgo', { n: days });
  if (days < 365) return t('format.monthsAgo', { n: Math.round(days / 30) });
  return t('format.yearsAgo', { n: Math.round(days / 365) });
}

// The list shows dates the way the Dateiliste settings ask for; every other place keeps the plain date.
export const fmtListDate = (ms: number) => ({ long: fmtTime, relative: fmtAgo }[prefs.dateFormat as string] || fmtDate)(ms);

export const fmtLeft = (s: number) => s < 60 ? t('format.secondsLeft', { n: Math.max(1, Math.ceil(s)) })
  : s < 3600 ? t('format.minutesLeft', { n: Math.ceil(s / 60) })
  : t('format.hoursLeft', { n: (s / 3600).toLocaleString(locale(), { maximumFractionDigits: 1 }) });

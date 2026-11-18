// The sums and shapes of the drives view: how full a drive is, its sparkline, the chart of the weeks shown and what
// the server's trend says about when it is full.
import type { UnraidDrive, UnraidSample } from '@/api/unraid';
import { DAY, fmtDate, fmtTime, now } from '@/core/format';
import { locale } from '@/i18n';

// From here on the meter turns red.
export const FULL = 0.95;
export const RANGES = [4, 8, 12] as const;
export type Range = (typeof RANGES)[number];

export const fillOf = (d: UnraidDrive) => (d.fsSize ? d.used / d.fsSize : 0);
const today = () => Math.floor(now() / DAY);

export function inRange(history: UnraidSample[], weeks: number) {
  const from = today() - weeks * 7;
  return history.filter(([day]) => day > from);
}

// The bytes gained or freed over the weeks shown, or nothing while there are not two days to compare.
export function changeOf(history: UnraidSample[], weeks: number): number | null {
  const pts = inRange(history, weeks);
  return pts.length > 1 ? pts.at(-1)![1] - pts[0]![1] : null;
}

// The server leaves out a trend that is flat or falls, so a missing key means saying nothing.
export function fullInWeeks(d: UnraidDrive, weeks: Range): number | null {
  const w = d.fullIn?.[String(weeks) as `${Range}`];
  return w == null ? null : Math.max(1, Math.round(w));
}

// Eight weeks of fill level in a box of 100 by 28, stretched to the drive's own range so a small change still shows.
export function sparkOf(d: UnraidDrive) {
  const h = inRange(d.history, 8).map(([, used]) => (d.fsSize ? used / d.fsSize : 0));
  if (h.length < 2) return null;
  const lo = Math.min(...h), span = Math.max(0.02, Math.max(...h) - lo);
  const line = h.map((v, i) => `${i ? 'L' : 'M'}${((i / (h.length - 1)) * 100).toFixed(1)} ${(26 - ((v - lo) / span) * 22).toFixed(1)}`).join(' ');
  return { line, area: `${line} L100 28 L0 28 Z` };
}

export interface Chart { line: string; area: string; ticks: { y: number; label: number }[]; marks: { x: number; day: number; anchor: string }[]; pts: { x: number; y: number; day: number; v: number }[] }
export const PAD = { l: 40, r: 8, t: 8, b: 22 };

// The weeks shown as a line of fill level over the calendar, with a grid every five or ten percent.
export function chartOf(d: UnraidDrive, weeks: Range, w: number, h: number): Chart | null {
  const pts = inRange(d.history, weeks).map(([day, used]) => ({ day, v: d.fsSize ? used / d.fsSize : 0 }));
  if (pts.length < 2) return null;
  const end = today(), start = end - weeks * 7;
  const vs = pts.map((p) => p.v);
  const lo = Math.max(0, Math.floor((Math.min(...vs) - 0.04) * 20) / 20), hi = Math.min(1, Math.ceil((Math.max(...vs) + 0.02) * 20) / 20);
  const X = (day: number) => PAD.l + ((day - start) / (end - start)) * (w - PAD.l - PAD.r);
  const Y = (v: number) => PAD.t + (1 - (v - lo) / (hi - lo || 1)) * (h - PAD.t - PAD.b);
  const step = hi - lo <= 0.2 ? 0.05 : 0.1;
  const ticks = [];
  for (let v = lo; v <= hi + 1e-9; v += step) ticks.push({ y: Math.round(Y(v)) + 0.5, label: Math.round(v * 100) });
  const marks = [];
  for (let i = 0; i <= weeks; i += weeks > 8 ? 2 : 1) marks.push({ x: X(start + i * 7), day: start + i * 7, anchor: i === 0 ? 'start' : i === weeks ? 'end' : 'middle' });
  const at = pts.map((p) => ({ ...p, x: X(p.day), y: Y(p.v) }));
  const line = at.map((p, i) => `${i ? 'L' : 'M'}${p.x.toFixed(1)} ${p.y.toFixed(1)}`).join(' ');
  return { line, area: `${line} L${at.at(-1)!.x.toFixed(1)} ${Y(lo).toFixed(1)} L${at[0]!.x.toFixed(1)} ${Y(lo).toFixed(1)} Z`, ticks, marks, pts: at };
}

export const dayText = (day: number, opts: Intl.DateTimeFormatOptions = { weekday: 'short', day: '2-digit', month: '2-digit' }) => new Date(day * DAY).toLocaleDateString(locale(), { ...opts, timeZone: 'UTC' });

// A moment of today as its time of day, any other with its date.
export function clockOf(iso: string) {
  const ms = Date.parse(iso);
  if (new Date(ms).toDateString() !== new Date(now()).toDateString()) return fmtTime(ms);
  return new Date(ms).toLocaleTimeString(locale(), { hour: '2-digit', minute: '2-digit' });
}

export const dateOf = (iso: string) => fmtDate(Date.parse(iso));

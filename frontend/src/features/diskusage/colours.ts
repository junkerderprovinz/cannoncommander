// The treemap's colours by file type and by age. The values live in diskusage.css as tokens that follow the theme
// and the accent, so a tile only names its token here.
import type { UsageInner } from '@/api/tools';

export type ColourMode = 'type' | 'age';
export type ColourKey = string;

export const TYPES = ['video', 'photo', 'music', 'docs', 'archive', 'other'] as const;
export type TypeKey = (typeof TYPES)[number];
// The server's kinds of file, folded into the six the legend has room for.
const TYPE_OF: Record<string, TypeKey> = { video: 'video', img: 'photo', audio: 'music', doc: 'docs', text: 'docs', arch: 'archive' };

// The ages the server counts bytes in: changed within a week, a month, six months, a year, three years, or before.
export const AGES = ['week', 'month', 'half', 'year', 'years', 'older'] as const;

export function typeMix(kinds: Record<string, number> = {}): [TypeKey, number][] {
  const sum = new Map<TypeKey, number>();
  for (const [k, v] of Object.entries(kinds)) if (v > 0) sum.set(TYPE_OF[k] ?? 'other', (sum.get(TYPE_OF[k] ?? 'other') ?? 0) + v);
  return [...sum].sort((a, b) => b[1] - a[1] || TYPES.indexOf(a[0]) - TYPES.indexOf(b[0]));
}

export const typeKey = (e: UsageInner): TypeKey => typeMix(e.kinds)[0]?.[0] ?? 'other';

// The age that holds the most bytes; an entry without any counts as old.
export function ageKey(e: UsageInner): ColourKey {
  const ages = e.ages ?? [];
  let best = AGES.length - 1;
  for (let i = 0; i < ages.length; i++) if ((ages[i] ?? 0) > (ages[best] ?? 0)) best = i;
  return String(best);
}

export const keyOf = (mode: ColourMode, e: UsageInner) => (mode === 'type' ? typeKey(e) : ageKey(e));
export const fillOf = (mode: ColourMode, key: ColourKey) => `var(--du-${mode}-${key})`;
export const inkOf = (mode: ColourMode, key: ColourKey) => `var(--du-ink-${mode}-${key})`;
export const dimmed = (mode: ColourMode, e: UsageInner, hl: ColourKey | null) => hl != null && keyOf(mode, e) !== hl;

// What each colour holds in the folder. By type it counts every byte of that type, by age every step is listed,
// so the ramp reads the same from folder to folder.
export function legend(mode: ColourMode, entries: UsageInner[]): { key: ColourKey; v: number }[] {
  if (mode === 'age') {
    const sum = AGES.map(() => 0);
    for (const e of entries) (e.ages ?? []).forEach((v, i) => { if (i < sum.length) sum[i]! += v; });
    return sum.map((v, i) => ({ key: String(i), v }));
  }
  const sum = new Map<TypeKey, number>(TYPES.map((k) => [k, 0]));
  for (const e of entries) for (const [k, v] of typeMix(e.kinds)) sum.set(k, sum.get(k)! + v);
  return TYPES.filter((k) => sum.get(k)! > 0).map((key) => ({ key, v: sum.get(key)! }));
}

// The hue of a #rrggbb colour in degrees, or null for a grey or anything else.
function hueOf(hex: string): number | null {
  const m = /^#?([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i.exec(hex.trim());
  if (!m) return null;
  const [r, g, b] = m.slice(1).map((x) => parseInt(x, 16) / 255) as [number, number, number];
  const max = Math.max(r, g, b), d = max - Math.min(r, g, b);
  if (d < 0.08) return null;
  const h = max === r ? ((g - b) / d) % 6 : max === g ? (b - r) / d + 2 : (r - g) / d + 4;
  return (h * 60 + 360) % 360;
}

// A cyan to blue accent would make the age ramp look like the video blue, so the ramp then starts from sunflower.
export function ageBase(accent: string): 'accent' | 'warm' {
  const h = hueOf(accent);
  return h != null && h >= 170 && h <= 255 ? 'warm' : 'accent';
}

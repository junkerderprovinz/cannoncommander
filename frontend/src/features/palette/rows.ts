// What the command search lists: every action by its name, the recent ones and the frequent folders on top while
// nothing is typed, and the best matches first once something is.
import { type Action, actions, blocked, labelOf } from '@/core/actions';
import type { MenuEntry } from '@/core/menu';
import { MENUS, menuName } from '@/core/menus';
import { byName, byText, t } from '@/i18n';

export interface Row { name: string; g?: string; a?: Action; group?: string; why?: string | null; go?: () => void; score?: number }
export interface Head { head: string }
export type Line = Row | Head;
export interface Source { head: () => string; list: () => Row[] }

const KEY = 'cannoncommander-recent-commands';
export const sources: Source[] = [];

const recent = (): string[] => {
  try { const v = JSON.parse(localStorage.getItem(KEY) || '[]'); return Array.isArray(v) ? v : []; } catch { return []; }
};
export function remember(id: string) {
  try { localStorage.setItem(KEY, JSON.stringify([id, ...recent().filter((x) => x !== id)].slice(0, 6))); } catch { /* the list just stays as it was */ }
}

const text = (v: unknown) => (typeof v === 'function' ? v() : String(v));

// A bare label such as Name only makes sense with the menu or heading it sits under.
function groups() {
  const out: Record<string, string> = {};
  MENUS.forEach(([, items], i) => {
    let head = menuName(i);
    for (const it of items as MenuEntry[]) {
      if (it === '-') head = menuName(i);
      else if (typeof it === 'object' && 'head' in it) head = text(it.head);
      else if (typeof it === 'string' && !out[it]) out[it] = head;
    }
  });
  return out;
}

const fold = (s: string) => s.toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/ß/g, 'ss');
const inOrder = (want: string, s: string) => { let i = 0; for (const c of s) if (c === want[i]) i++; return i === want.length; };

// Every typed word has to show up somewhere; the start of a word counts most, letters in order least.
export function score(q: string, e: Pick<Row, 'name' | 'group'>) {
  const tokens = fold(q).split(/\s+/).filter(Boolean);
  const name = fold(e.name), words = ' ' + name.replace(/[^a-z0-9]+/g, ' '), group = fold(e.group || '');
  let s = name.startsWith(tokens.join(' ')) ? 3 : 0;
  for (const tk of tokens) {
    const hit = words.includes(' ' + tk) ? 3 : name.includes(tk) ? 2 : group.includes(tk) || inOrder(tk, name) ? 1 : 0;
    if (!hit) return 0;
    s += hit;
  }
  return s;
}

export function lines(q: string): Line[] {
  const grp = groups();
  const extra = sources.map((s) => ({ head: s.head(), rows: s.list().map((r) => ({ ...r, group: s.head() })) }));
  const all: Row[] = Object.values(actions).filter((a) => labelOf(a) && !a.hidden && a.id !== 'palette').map((a) => ({
    a, name: labelOf(a), g: a.g, group: grp[a.id], why: blocked(a) || (a.when && !a.when() ? t('palette.notNow') : null),
  }));
  const byName = (x: Row, y: Row) => Number(!!x.why) - Number(!!y.why) || byText(x.name, y.name);
  if (!q.trim()) {
    const first = recent().map((id) => all.find((e) => e.a!.id === id)).filter((e): e is Row => !!e);
    const rest = all.filter((e) => !first.includes(e)).sort(byName);
    const more = extra.filter((x) => x.rows.length).flatMap((x) => [{ head: x.head }, ...x.rows.slice(0, 5)]);
    return first.length || more.length
      ? [...(first.length ? [{ head: t('palette.recent') }, ...first] : []), ...more, { head: t('palette.all') }, ...rest]
      : rest;
  }
  return [...all, ...extra.flatMap((x) => x.rows)].map((e) => ({ ...e, score: score(q, e) })).filter((e) => e.score)
    .sort((x, y) => Number(!!x.why) - Number(!!y.why) || y.score - x.score || byName(x, y));
}

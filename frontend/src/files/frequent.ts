// Folders ranked like zoxide: how often, weighted by how recently, kept in this browser.
const KEY = 'cannoncommander-frecent';
export interface Visit { n: number; t: number }

export const rank = (v: Visit, now: number) => {
  const h = (now - v.t) / 3600000;
  return v.n * (h < 1 ? 4 : h < 24 ? 2 : h < 168 ? 0.5 : 0.25);
};

function read(): Record<string, Visit> {
  try { return JSON.parse(localStorage.getItem(KEY) || '{}') || {}; } catch { return {}; }
}

export function recordVisit(path: string, now = Date.now()) {
  const all = read(), v = all[path] || { n: 0, t: 0 };
  all[path] = { n: v.n + 1, t: now };
  try { localStorage.setItem(KEY, JSON.stringify(all)); } catch { /* the ranking just stays where it was */ }
}

export function topFolders(n = 8, skip = '', now = Date.now(), visits = read()) {
  return Object.entries(visits).filter(([p]) => p !== skip).sort((a, b) => rank(b[1], now) - rank(a[1], now)).slice(0, n).map(([p]) => p);
}

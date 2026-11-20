// The welcome window: the logo, a greeting and the ways to lay out the app, each one taking effect the moment it is
// picked, so the panels behind the window already show it.
import raw from '@/assets/brand/cannoncommander.svg?raw';
import { host } from '@/core/host';
import { firstVisit, prefs, setPref } from '@/core/prefs';
import { useStyle } from '@/features/desktop/style';
import { t } from '@/i18n';

export type Way = 'explorer' | 'commander' | 'single' | 'dual';
export interface Choice { id: Way; name: string; how: string; panes: 1 | 2; fkeys: boolean }

// In the desktop app a way brings its keys and its kind of selecting along. In the browser those keys belong to the
// browser, so only the panels are left to pick.
export function choices(): Choice[] {
  return host.desktop
    ? [
      { id: 'explorer', name: t('desktop.explorerName'), how: t('desktop.explorerHow'), panes: 1, fkeys: false },
      { id: 'commander', name: t('desktop.commanderName'), how: t('desktop.commanderHow'), panes: 2, fkeys: true },
    ]
    : [
      { id: 'single', name: t('actions.single'), how: t('welcome.singleHow'), panes: 1, fkeys: false },
      { id: 'dual', name: t('actions.dual'), how: t('welcome.dualHow'), panes: 2, fkeys: true },
    ];
}

export function chosen(): Way {
  if (host.desktop) return prefs.desktopStyle === 'explorer' ? 'explorer' : 'commander';
  return prefs.dual ? 'dual' : 'single';
}

export function choose(way: Way) {
  if (way === 'explorer' || way === 'commander') useStyle(way);
  else setPref('dual', way === 'dual');
}

export const welcomeDue = () => firstVisit() && !prefs.welcomed;

// The master logo inline, so its two cannons can kick back on their own. Its stylesheet would reach the whole page
// once inline, so the colours move onto the shapes. The upper cannon fires to the right, the lower one to the left.
export function welcomeLogo(): string {
  const doc = new DOMParser().parseFromString(raw, 'image/svg+xml');
  const root = doc.documentElement;
  const fills = new Map([...(root.querySelector('style')?.textContent ?? '').matchAll(/\.([\w-]+)\s*\{\s*fill:\s*([^;}\s]+)/g)].map((m) => [m[1]!, m[2]!]));
  root.querySelector('defs')?.remove();
  for (const el of root.querySelectorAll('[class]')) {
    const fill = fills.get(el.getAttribute('class')!);
    if (fill) el.setAttribute('fill', fill);
    el.removeAttribute('class');
  }
  const low = root.querySelector('#kannonen')!;
  const high = root.querySelector('#bombe')!.nextElementSibling!;
  low.setAttribute('class', 'wl-cannon wl-cannon--low');
  high.setAttribute('class', 'wl-cannon wl-cannon--high');
  const puff = (x: number, y: number, side: string) => {
    const el = doc.createElementNS('http://www.w3.org/2000/svg', 'circle');
    Object.entries({ cx: x, cy: y, r: 46, class: `wl-puff wl-puff--${side}` }).forEach(([k, v]) => el.setAttribute(k, String(v)));
    return el;
  };
  low.parentElement!.append(puff(800, 303, 'high'), puff(200, 697, 'low'));
  for (const named of [root, ...root.querySelectorAll('[id], [data-name]')]) {
    named.removeAttribute('id');
    named.removeAttribute('data-name');
  }
  root.setAttribute('aria-hidden', 'true');
  root.setAttribute('focusable', 'false');
  return new XMLSerializer().serializeToString(root).replace(/ xmlns="[^"]*"/g, '');
}

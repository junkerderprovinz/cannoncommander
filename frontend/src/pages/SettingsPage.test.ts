import { afterEach, beforeAll, describe, expect, it } from 'vitest';
import { createApp, defineComponent, h, nextTick, ref } from 'vue';
import { host } from '@/core/host';
import { settingsTab, settingsTabs } from '@/core/views';
import { closing, sheets } from '@/testing/motionCss';
import SettingsPage from './SettingsPage.vue';

const body = (id: string) => defineComponent({ render: () => h('p', { 'data-body': id }) });

beforeAll(() => {
  settingsTabs.splice(0);
  settingsTab({ id: 'general', label: 'Allgemein', g: 'IconTabGeneral', order: 10, component: body('general') });
  settingsTab({ id: 'look', label: 'Aussehen', g: 'IconTabLook', order: 20, component: body('look') });
  settingsTab({ id: 'security', label: 'Konto', g: 'IconTabSecurity', order: 30, component: body('security'), hidden: () => host.desktop });
  settingsTab({ id: 'users', label: 'Benutzer', g: 'IconAccounts', order: 70, component: body('users'), hidden: () => host.desktop });
});

let unmount = () => {};
afterEach(() => {
  unmount();
  document.body.innerHTML = '';
  host.desktop = false;
});

async function open(arg?: string) {
  const el = document.createElement('div');
  document.body.append(el);
  const app = createApp(SettingsPage, { arg });
  app.mount(el);
  unmount = () => app.unmount();
  await nextTick();
}
const tiles = () => [...document.querySelectorAll<HTMLElement>('.stile')].map((t) => t.dataset.stab);
const shown = () => document.querySelector<HTMLElement>('[data-body]')?.dataset.body;

describe('the settings in the desktop app', () => {
  it('leave out the tiles about signing in and other people, Allgemein and Aussehen first', async () => {
    host.desktop = true;
    await open();
    expect(tiles()).toEqual(['general', 'look']);
  });

  it('open Allgemein for a link to a tile they leave out', async () => {
    host.desktop = true;
    await open('users');
    expect(shown()).toBe('general');
  });

  it('keep every tile in the container', async () => {
    await open('users');
    expect(tiles()).toEqual(['general', 'look', 'security', 'users']);
    expect(shown()).toBe('users');
  });
});

describe('the settings moving', () => {
  async function openAt(arg: string) {
    const at = ref(arg), el = document.createElement('div');
    document.body.append(el);
    const app = createApp({ render: () => h(SettingsPage, { arg: at.value }) });
    app.mount(el);
    unmount = () => app.unmount();
    await nextTick();
    return at;
  }
  const col = () => document.querySelector<HTMLElement>('#settings-col')!;

  it('slide the content in from the side of the tile picked, the trailing side for a tile further down', async () => {
    const at = await openAt('look');
    at.value = 'users';
    await nextTick();
    expect(col().classList).toContain('glim-tab-slide');
    expect(col().style.getPropertyValue('--tab-dir')).toBe('1');
    at.value = 'general';
    await nextTick();
    expect(col().style.getPropertyValue('--tab-dir')).toBe('-1');
  });

  it('bring the tiles in one after another when the page opens', async () => {
    await openAt('general');
    const tiles = [...document.querySelectorAll<HTMLElement>('.stile')];
    expect(tiles.every((t) => t.classList.contains('glim-stagger-row'))).toBe(true);
    expect(tiles.map((t) => t.style.getPropertyValue('--row-i'))).toEqual(['0', '1', '2', '3']);
  });
});

// jsdom lays nothing out and skips every rule behind a width query, so the cascade is worked out here from the
// sheets that draw the settings, in the order the app loads them: the most specific rule wins, then the last one.
const ORDER = ['styles/app.css', 'styles/shell.css', 'features/config/config.css'];
interface Rule { media: string; selector: string; body: string }
function flatten(css: string, media = ''): Rule[] {
  const out: Rule[] = [];
  const re = /@media([^{]*)\{|([^{}]+)\{([^{}]*)\}/g;
  for (let m = re.exec(css); m; m = re.exec(css)) {
    if (m[1] === undefined) {
      out.push({ media, selector: m[2]!.trim(), body: m[3]! });
      continue;
    }
    const end = closing(css, m.index);
    out.push(...flatten(css.slice(m.index + m[0].length, end), m[1].trim()));
    re.lastIndex = end + 1;
  }
  return out;
}
const RULES = ORDER.flatMap((file) => flatten(sheets().find((s) => s.file === file)!.css));

const fits = (media: string, width: number) => !media || media.split(/\s+and\s+/).every((q) => {
  const m = /^\((min|max)-width:\s*([\d.]+)(px|rem)\)$/.exec(q.trim());
  if (!m) return false;
  const px = Number(m[2]) * (m[3] === 'rem' ? 16 : 1);
  return m[1] === 'min' ? width >= px : width <= px;
});
function parts(list: string): string[] {
  const out = [''];
  let depth = 0;
  for (const c of list) {
    if (c === '(') depth++;
    if (c === ')') depth--;
    if (c === ',' && depth === 0) out.push('');
    else out[out.length - 1] += c;
  }
  return out.map((s) => s.trim());
}
function specificity(selector: string): number {
  let n = 0;
  const rest = selector.replace(/:(is|not|has|where)\(((?:[^()]|\([^()]*\))*)\)/g, (_, fn: string, inner: string) => {
    if (fn !== 'where') n += Math.max(...parts(inner).map(specificity));
    return ' ';
  }).replace(/\[[^\]]*\]/g, () => { n += 100; return ' '; });
  n += (rest.match(/#[\w-]+/g)?.length ?? 0) * 10000;
  n += (rest.match(/\.[\w-]+|:(?!:)[\w-]+/g)?.length ?? 0) * 100;
  n += rest.match(/(^|[\s>+~])[a-z][\w-]*/gi)?.length ?? 0;
  return n;
}
const matches = (el: Element, selector: string) => {
  // Pseudo-elements and a few newer selectors are past jsdom's parser, and none of them styles the element itself.
  try { return el.matches(selector); } catch { return false; }
};
const outranks = (a: number[], b: number[]) => {
  const i = a.findIndex((x, k) => x !== b[k]);
  return i >= 0 && a[i]! > b[i]!;
};
function style(el: Element, prop: string, width: number): string {
  let best: { rank: number[]; value: string } | undefined;
  RULES.forEach((rule, order) => {
    if (!fits(rule.media, width)) return;
    const decl = rule.body.split(';').map((d) => d.split(':')).findLast(([k]) => k!.trim() === prop);
    if (!decl) return;
    const value = decl.slice(1).join(':').trim();
    const important = value.endsWith('!important');
    for (const sel of parts(rule.selector)) {
      if (!matches(el, sel)) continue;
      const rank = [important ? 1 : 0, specificity(sel), order];
      if (!best || outranks(rank, best.rank)) best = { rank, value: value.replace(/\s*!important$/, '') };
    }
  });
  return best?.value ?? '';
}

describe('the settings tiles', () => {
  const MODES = ['text', 'textGlyph', 'glyph', 'reactive'];
  const at = async (mode: string) => {
    document.documentElement.setAttribute('data-labels-tabs', mode);
    await open('users');
    const tile = document.querySelector<HTMLElement>('.stile:not(.cur)')!;
    return { settings: document.querySelector('.settings')!, tile, glyph: tile.querySelector('svg')!, name: tile.querySelector('span')! };
  };
  afterEach(() => document.documentElement.removeAttribute('data-labels-tabs'));

  it('narrow to a column of glyphs on a wide window when they show glyphs alone', async () => {
    const { settings, tile, name } = await at('glyph');
    expect(style(settings, 'grid-template-columns', 1600)).toBe('3.25rem minmax(0, 1fr)');
    expect(style(tile, 'justify-content', 1600)).toBe('center');
    expect(style(tile, 'padding', 1600)).toBe('0');
    expect(style(name, 'display', 1600)).toBe('none');
  });

  it.each(['text', 'textGlyph', 'reactive'])('keep the room for the names on a wide window in %s', async (mode) => {
    const { settings } = await at(mode);
    expect(style(settings, 'grid-template-columns', 1600)).toBe('12.75rem minmax(0, 1fr)');
  });

  it('drop their glyphs on a wide window when they show words alone', async () => {
    const { glyph } = await at('text');
    expect(style(glyph, 'display', 1600)).toBe('none');
    document.documentElement.setAttribute('data-labels-tabs', 'textGlyph');
    expect(style(glyph, 'display', 1600)).not.toBe('none');
  });

  it.each(MODES.flatMap((mode) => [700, 900, 1024].map((width) => [mode, width] as const)))(
    'show their glyphs alone in a narrow column in %s at %ipx', async (mode, width) => {
      const { settings, tile, glyph, name } = await at(mode);
      expect(style(settings, 'grid-template-columns', width)).toBe('3.25rem minmax(0, 1fr)');
      expect(style(tile, 'justify-content', width)).toBe('center');
      expect(style(glyph, 'display', width)).not.toBe('none');
      expect(style(name, 'display', width)).toBe('none');
    });

  it.each(MODES)('run in a strip above the page on a phone in %s', async (mode) => {
    const { settings, tile } = await at(mode);
    expect(style(settings, 'grid-template-columns', 420)).toBe('minmax(0, 1fr)');
    expect(style(tile, 'width', 420)).toBe('2.75rem');
  });

  it('keep every name for the bubble and the screen reader when only the glyph shows', async () => {
    await at('glyph');
    const named = [...document.querySelectorAll<HTMLElement>('.stile')].map((t) => [t.dataset.tip, t.getAttribute('aria-label')]);
    expect(named).toEqual([['Allgemein', 'Allgemein'], ['Aussehen', 'Aussehen'], ['Konto', 'Konto'], ['Benutzer', 'Benutzer']]);
  });
});

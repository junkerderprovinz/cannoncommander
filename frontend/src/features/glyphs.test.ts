// Every command and every menu row carries a glyph, so no entry stands in a list of pictures as a bare word. The
// app is set up as it starts, with every feature, so an action added later is checked without being named here.
import { type MenuEntry as GlimEntry, menuRowView } from '@glimstone/core';
import { createPinia, setActivePinia } from 'pinia';
import { beforeAll, describe, expect, it } from 'vitest';
import { createApp, nextTick } from 'vue';
import { actions, labelOf } from '@/core/actions';
import { hasGlyph } from '@/core/glyphs';
import { type MenuEntry, menuState, resolveItems, toGlim } from '@/core/menu';
import { allMenuItems, MENUS, menuItems, menuName } from '@/core/menus';
import { loadFeatures } from '@/features';
import Palette from '@/features/palette/Palette.vue';
import { lines } from '@/features/palette/rows';
import { pal } from '@/features/palette/state';
import { registerFileActions } from '@/files/actions';
import { listMenu, rowMenu, tabMenu } from '@/files/menus';
import { VIEWS } from '@/files/views';
import { usePanels } from '@/stores/panels';

beforeAll(() => {
  setActivePinia(createPinia());
  usePanels();
  registerFileActions();
  loadFeatures();
});

type Row = Exclude<GlimEntry<unknown>, '-' | { head: string }>;
const isRow = (e: GlimEntry<unknown>): e is Row => e !== '-' && !('head' in e);

// A row without a glyph, looked for in its side menus too.
function bare(entries: GlimEntry<unknown>[], path = ''): string[] {
  return entries.filter(isRow).flatMap((e) => {
    const name = `${path}${e.label}`;
    const lead = menuRowView(e).lead;
    return [...(lead === 'glyph' || lead === 'dot' ? [] : [`${name} (${lead})`]), ...(e.sub ? bare(e.sub, `${name} > `) : [])];
  });
}
const shown = (items: MenuEntry[]) => toGlim(resolveItems(items));
const opened = (open: () => void) => {
  open();
  return toGlim(menuState.list ?? []);
};

describe('glyphs', () => {
  it('name a glyph GlimStone has for every action', () => {
    const missing = Object.values(actions).filter((a) => !(a.g && hasGlyph(a.g))).map((a) => `${a.id}: ${a.g ?? 'none'}`);
    expect(missing).toEqual([]);
  });

  for (const [i, [id]] of MENUS.entries()) {
    it(`stand before every entry of the ${id} menu`, () => {
      expect(bare(shown(menuItems(i, [{ label: 'Downloads', g: 'IconDownload', run: () => {} }])), `${menuName(i)} > `)).toEqual([]);
    });
  }

  it('stand before every entry when the menus stack into one list', () => {
    expect(bare(shown(allMenuItems([])))).toEqual([]);
  });

  it('stand before every entry of the context menus and the view menu', () => {
    const F = usePanels();
    F.T().marked = new Set(['a.txt']);
    expect(bare(opened(() => rowMenu(0, 0)))).toEqual([]);
    expect(bare(opened(() => listMenu(0, 0)))).toEqual([]);
    expect(bare(opened(() => tabMenu(0, 0)))).toEqual([]);
    expect(bare(shown(VIEWS.map(([v]) => 'view-' + v)))).toEqual([]);
  });

  it('stand before every command in the search', () => {
    const rows = lines('').filter((r) => !('head' in r));
    expect(rows.length).toBeGreaterThan(50);
    expect(rows.filter((r) => !('head' in r) && !(r.g && hasGlyph(r.g))).map((r) => ('name' in r ? r.name : ''))).toEqual([]);
  });

  it('lead every row of the command search, a switch too', async () => {
    pal.open = true;
    const el = document.body.appendChild(document.createElement('div'));
    const app = createApp(Palette);
    app.mount(el);
    await nextTick();
    const rows = [...el.querySelectorAll('.pal-row')];
    expect(rows.length).toBeGreaterThan(50);
    expect(rows.filter((r) => r.firstElementChild?.tagName !== 'svg').map((r) => r.querySelector('.t')?.textContent)).toEqual([]);
    app.unmount();
    pal.open = false;
  });

  it('are not swapped for a tick box in a menu, which shows a state rather than the command', () => {
    expect(bare(shown(['single', 'dual', 'sidebar', 'sortType']))).toEqual([]);
    expect(labelOf(actions.sortType!)).toBe('Dateityp');
  });
});

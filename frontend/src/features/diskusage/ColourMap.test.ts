import { afterEach, beforeAll, describe, expect, it } from 'vitest';
import { createApp, h, nextTick, reactive } from 'vue';
import type { UsageEntry } from '@/api/tools';
import { addMessages } from '@/i18n';
import ColourMap from './ColourMap.vue';
import de from './de.json';

const MB = 1 << 20;
const file = (name: string, size: number, kinds: Record<string, number>, ages: number[]) => ({ name, dir: false, size, files: 1, dirs: 0, mtime: '', kinds, ages });
const items: UsageEntry[] = [
  { name: 'filme', dir: true, size: 600 * MB, files: 3, dirs: 1, mtime: '', kinds: { video: 590 * MB, text: 10 * MB }, ages: [0, 0, 0, 0, 600 * MB, 0], more: 2,
    items: [{ ...file('Klassiker', 400 * MB, { video: 400 * MB }, [0, 0, 0, 0, 400 * MB, 0]), dir: true }, file('neu.mkv', 150 * MB, { video: 150 * MB }, [150 * MB, 0, 0, 0, 0, 0])] },
  { name: 'fotos', dir: true, size: 300 * MB, files: 2, dirs: 0, mtime: '', kinds: { img: 300 * MB }, ages: [300 * MB, 0, 0, 0, 0, 0],
    items: [file('a.jpg', 200 * MB, { img: 200 * MB }, [200 * MB, 0, 0, 0, 0, 0]), file('b.jpg', 100 * MB, { img: 100 * MB }, [100 * MB, 0, 0, 0, 0, 0])] },
  file('debian.iso', 100 * MB, { disk: 100 * MB }, [0, 0, 0, 0, 0, 100 * MB]),
];

beforeAll(() => addMessages('diskusage', { de }));
let app: ReturnType<typeof createApp> | undefined;
afterEach(() => {
  app?.unmount();
  document.body.innerHTML = '';
});

async function mount(over: Partial<{ mode: 'type' | 'age'; hl: string; picked: Set<string> }> = {}) {
  const props = reactive({ items, path: '/mnt/user', mode: 'type' as 'type' | 'age', hl: null as string | null, picked: new Set<string>(), w: 1000, h: 600, ...over });
  const heard: { go: string[]; pick: [number, boolean][] } = { go: [], pick: [] };
  app = createApp({ render: () => h(ColourMap, { ...props, onGo: (p: string) => heard.go.push(p), onPick: (i: number, e: MouseEvent) => heard.pick.push([i, e.ctrlKey]) }) });
  app.mount(document.body.appendChild(document.createElement('div')));
  await nextTick();
  return { props, heard };
}
const groups = () => [...document.querySelectorAll<SVGGElement>('.du-group')];
const tiles = () => [...document.querySelectorAll<SVGGElement>('.du-tile')];
const fill = (g: Element) => (g as SVGGElement).style.getPropertyValue('--blk');

describe('the coloured treemap', () => {
  it('draws each folder as a group of its own entries and a loose file as a tile', async () => {
    await mount();
    expect(groups().map((g) => g.dataset.name)).toEqual(['filme', 'fotos']);
    expect(tiles().map((t) => t.dataset.name)).toEqual(['Klassiker', 'neu.mkv', '', 'a.jpg', 'b.jpg', 'debian.iso']);
  });

  it('colours every tile by its largest type', async () => {
    await mount();
    expect(tiles().map(fill)).toEqual(['var(--du-type-video)', 'var(--du-type-video)', 'var(--carbon-surface3)', 'var(--du-type-photo)', 'var(--du-type-photo)', 'var(--du-type-other)']);
  });

  it('colours every tile by the age that holds most of it', async () => {
    await mount({ mode: 'age' });
    expect(tiles().map(fill)).toEqual(['var(--du-age-4)', 'var(--du-age-0)', 'var(--carbon-surface3)', 'var(--du-age-0)', 'var(--du-age-0)', 'var(--du-age-5)']);
  });

  it('dims the tiles of every other colour while one is highlighted', async () => {
    await mount({ hl: 'photo' });
    expect(tiles().map((t) => t.classList.contains('dim'))).toEqual([true, true, true, false, false, true]);
  });

  it('labels the rest of a folder by how many entries it holds', async () => {
    await mount();
    tiles()[2]!.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, clientX: 10, clientY: 10 }));
    await nextTick();
    expect(document.querySelector('.du-tip b')?.textContent).toBe('2 kleinere Einträge');
  });

  it('tells the path, the share and the mix of a tile under the pointer', async () => {
    await mount();
    tiles()[0]!.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, clientX: 10, clientY: 10 }));
    await nextTick();
    const tip = document.querySelector('.du-tip')!;
    expect(tip.querySelector('b')?.textContent).toBe('Klassiker');
    expect(tip.querySelector('.path')?.textContent).toBe('/mnt/user/filme/Klassiker');
    expect(tip.textContent).toContain('67 % von filme');
    expect([...tip.querySelectorAll('.du-mix > span:not(.glim-num)')].map((x) => x.textContent)).toEqual(['Video']);
    expect(tip.textContent).toContain('Das meiste davon ist unter 3 Jahren alt');
    tiles()[0]!.dispatchEvent(new PointerEvent('pointerleave', { bubbles: true }));
    await nextTick();
    expect(document.querySelector('.du-tip')).toBeNull();
  });

  it('goes into a folder on a click and picks it with Ctrl held', async () => {
    const { heard } = await mount();
    tiles()[3]!.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    groups()[0]!.dispatchEvent(new MouseEvent('click', { bubbles: true, ctrlKey: true }));
    tiles()[5]!.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    expect(heard.go).toEqual(['/mnt/user/fotos']);
    expect(heard.pick).toEqual([[0, true], [2, false]]);
  });

  it('marks the picked entries', async () => {
    await mount({ picked: new Set(['/mnt/user/fotos', '/mnt/user/debian.iso']) });
    expect(groups().map((g) => g.classList.contains('sel'))).toEqual([false, true]);
    expect(tiles().at(-1)!.classList).toContain('sel');
  });
});

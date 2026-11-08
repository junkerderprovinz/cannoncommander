// Reordering by dragging in the settings tiles and the jobs list: the carried item follows the pointer, the others
// make room while it passes, and the new order is written once, on the drop.
import { createPinia, setActivePinia } from 'pinia';
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import type { Job } from '@/api/ops';
import { prefs } from '@/core/prefs';
import { settingsTab } from '@/core/views';
import JobsBody from '@/features/jobs/JobsBody.vue';
import SettingsPage from '@/pages/SettingsPage.vue';
import { useJobs } from '@/stores/jobs';

// jsdom lays nothing out, so every item of a list is 40px tall and stands 10px below the one before it.
const ROW = 40;
const STEP = 50;
const listed = (el: HTMLElement) => !!el.parentElement?.matches('.tiles, .job-list');
const slot = (el: HTMLElement) => [...el.parentElement!.children].indexOf(el);
const layout: Record<string, (el: HTMLElement) => number> = {
  offsetTop: (el) => (listed(el) ? slot(el) * STEP : 0),
  offsetHeight: (el) => (listed(el) ? ROW : 0),
  offsetLeft: () => 0,
  offsetWidth: (el) => (listed(el) ? 200 : 0),
};
const native = Object.fromEntries(Object.keys(layout).map((k) => [k, Object.getOwnPropertyDescriptor(HTMLElement.prototype, k)!]));

function point(target: EventTarget, type: string, y: number, pointerType = 'mouse') {
  const e = new MouseEvent(type, { bubbles: true, cancelable: true, clientX: 10, clientY: y, button: 0 });
  Object.defineProperties(e, { pointerId: { value: 1 }, pointerType: { value: pointerType } });
  target.dispatchEvent(e);
}

let unmount = () => {};
async function mount(component: Parameters<typeof createApp>[0]) {
  const el = document.createElement('div');
  document.body.append(el);
  const app = createApp(component);
  app.mount(el);
  unmount = () => app.unmount();
  await nextTick();
}

const tileOrder = () => [...document.querySelectorAll<HTMLElement>('.tiles [data-stab]')].map((el) => el.dataset.stab);
const tile = (id: string) => document.querySelector<HTMLElement>(`[data-stab="${id}"]`)!;
const jobOrder = () => [...document.querySelectorAll<HTMLElement>('.job-list [data-job]')].map((el) => el.dataset.job);
const job = (id: string) => document.querySelector<HTMLElement>(`[data-job="${id}"]`)!;

// A tile is also a link to its page, so a mouse arms it by holding still, as a finger does.
async function liftTile(id: string, y: number) {
  point(tile(id), 'pointerdown', y);
  vi.advanceTimersByTime(300);
  await nextTick();
}

beforeAll(() => {
  for (const [k, get] of Object.entries(layout)) Object.defineProperty(HTMLElement.prototype, k, { configurable: true, get() { return get(this); } });
  Element.prototype.scrollIntoView ??= () => {};
  setActivePinia(createPinia());
  for (const [i, id] of ['security', 'general', 'look'].entries()) settingsTab({ id, label: id, g: 'IconGear', order: i, component: { render: () => null } });
  const jobs = useJobs();
  for (const [i, id] of ['a', 'b', 'c'].entries()) jobs.upsert({ id, kind: 'copy', state: 'queued', position: i, createdAt: '2026-11-07T10:00:00Z' } as Job);
});
afterAll(() => {
  for (const [k, d] of Object.entries(native)) Object.defineProperty(HTMLElement.prototype, k, d);
});
beforeEach(() => {
  vi.useFakeTimers();
  prefs.tabOrder = [];
});
afterEach(() => {
  unmount();
  document.body.innerHTML = '';
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe('a carried settings tile', () => {
  it('sets the other tiles trembling until it is let go', async () => {
    await mount(SettingsPage);
    const tiles = document.querySelector('.tiles')!;
    expect(tiles.classList).not.toContain('glim-drag-armed');
    await liftTile('general', 70);
    expect(tiles.classList).toContain('glim-drag-armed');
    expect(tile('general').classList).toContain('glim-drag-lift');
    expect(tiles.querySelectorAll('.glim-drag-lift')).toHaveLength(1);
    expect(tiles.classList).not.toContain('glim-drag-rows');
    point(document, 'pointerup', 70);
    await nextTick();
    expect(tiles.classList).not.toContain('glim-drag-armed');
  });

  it('does not lift on a short click', async () => {
    await mount(SettingsPage);
    point(tile('general'), 'pointerdown', 70);
    vi.advanceTimersByTime(100);
    point(document, 'pointerup', 70);
    vi.advanceTimersByTime(300);
    await nextTick();
    expect(document.querySelector('.glim-drag-lift')).toBeNull();
  });

  it('makes room live once its bottom edge passes the next tile\'s middle on the way down', async () => {
    await mount(SettingsPage);
    await liftTile('security', 20);
    // Carried down by 29px its bottom edge is at 69, one short of the middle of general (50 to 90).
    point(document, 'pointermove', 49);
    await nextTick();
    expect(tileOrder()).toEqual(['security', 'general', 'look']);
    point(document, 'pointermove', 51);
    await nextTick();
    expect(tileOrder()).toEqual(['general', 'security', 'look']);
    expect(tile('general').classList).toContain('glim-drag-shift');
    expect(prefs.tabOrder).toEqual([]);
    point(document, 'pointerup', 51);
    await nextTick();
    expect(prefs.tabOrder).toEqual(['general', 'security', 'look']);
  });

  it('passes the tile above once its top edge crosses that tile\'s middle', async () => {
    await mount(SettingsPage);
    await liftTile('look', 120);
    point(document, 'pointermove', 91);
    await nextTick();
    expect(tileOrder()).toEqual(['security', 'general', 'look']);
    point(document, 'pointermove', 89);
    await nextTick();
    expect(tileOrder()).toEqual(['security', 'look', 'general']);
  });

  it('stays put when carried back to where it was, and the drop writes nothing', async () => {
    await mount(SettingsPage);
    await liftTile('security', 20);
    point(document, 'pointermove', 60);
    await nextTick();
    point(document, 'pointermove', 22);
    await nextTick();
    expect(tileOrder()).toEqual(['security', 'general', 'look']);
    point(document, 'pointerup', 22);
    await nextTick();
    expect(prefs.tabOrder).toEqual([]);
  });

  it('puts everything back on Escape and writes nothing', async () => {
    await mount(SettingsPage);
    await liftTile('security', 20);
    point(document, 'pointermove', 120);
    await nextTick();
    expect(tileOrder()).toEqual(['general', 'look', 'security']);
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    await nextTick();
    expect(tileOrder()).toEqual(['security', 'general', 'look']);
    point(document, 'pointerup', 120);
    await nextTick();
    expect(prefs.tabOrder).toEqual([]);
  });

  it('does not open the carried tile\'s page on the click that ends the drag', async () => {
    await mount(SettingsPage);
    expect(tile('security').getAttribute('aria-selected')).toBe('true');
    await liftTile('general', 70);
    point(document, 'pointermove', 120);
    await nextTick();
    point(document, 'pointerup', 120);
    tile('general').dispatchEvent(new MouseEvent('click', { bubbles: true, detail: 1 }));
    await nextTick();
    expect(tile('security').getAttribute('aria-selected')).toBe('true');
  });
});

describe('a carried job', () => {
  it('sets the other jobs lifting gently, as full-width rows do, until it is let go', async () => {
    await mount(JobsBody);
    const list = document.querySelector('.job-list')!;
    expect(list.querySelectorAll(':scope > .job')).toHaveLength(3);
    point(job('b'), 'pointerdown', 70);
    point(document, 'pointermove', 76);
    await nextTick();
    expect(list.classList).toContain('glim-drag-armed');
    expect(list.classList).toContain('glim-drag-rows');
    expect([...list.querySelectorAll<HTMLElement>('.glim-drag-lift')].map((el) => el.dataset.job)).toEqual(['b']);
    point(document, 'pointerup', 76);
    await nextTick();
    expect(list.classList).not.toContain('glim-drag-armed');
  });

  it('moves the others aside while it passes and tells the server once, on the drop', async () => {
    const jobs = useJobs();
    const move = vi.spyOn(jobs, 'move').mockResolvedValue();
    await mount(JobsBody);
    point(job('a'), 'pointerdown', 20);
    point(document, 'pointermove', 51);
    await nextTick();
    expect(jobOrder()).toEqual(['b', 'a', 'c']);
    expect(move).not.toHaveBeenCalled();
    point(document, 'pointerup', 51);
    await nextTick();
    expect(move).toHaveBeenCalledTimes(1);
    expect(move.mock.calls[0]![0].id).toBe('a');
    expect(move.mock.calls[0]![1]).toBe(1);
  });

  it('leaves a press on one of its buttons to the button', async () => {
    await mount(JobsBody);
    point(job('a').querySelector('[data-jact="cancel"]')!, 'pointerdown', 20);
    point(document, 'pointermove', 60);
    await nextTick();
    expect(document.querySelector('.glim-drag-lift')).toBeNull();
  });
});

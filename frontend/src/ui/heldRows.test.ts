// While a row of a reorderable list is held, the other rows tremble: GlimStone draws that for every child of a list
// that carries glim-drag-armed.
import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeAll, describe, expect, it } from 'vitest';
import { createApp, nextTick } from 'vue';
import type { Job } from '@/api/ops';
import { settingsTab } from '@/core/views';
import JobsBody from '@/features/jobs/JobsBody.vue';
import SettingsPage from '@/pages/SettingsPage.vue';
import { useJobs } from '@/stores/jobs';

const drag = (el: Element, type: string) => {
  const e = new Event(type, { bubbles: true, cancelable: true });
  Object.defineProperty(e, 'dataTransfer', { value: { effectAllowed: '' } });
  el.dispatchEvent(e);
};

let unmount = () => {};
async function mount(component: Parameters<typeof createApp>[0]) {
  const el = document.createElement('div');
  document.body.append(el);
  const app = createApp(component);
  app.mount(el);
  unmount = () => app.unmount();
  await nextTick();
}

beforeAll(() => {
  Element.prototype.scrollIntoView ??= () => {};
  setActivePinia(createPinia());
  for (const [i, id] of ['security', 'general', 'look'].entries()) settingsTab({ id, label: id, g: 'IconGear', order: i, component: { render: () => null } });
  const jobs = useJobs();
  for (const id of ['a', 'b', 'c']) jobs.upsert({ id, kind: 'copy', state: 'queued', position: 0, createdAt: '2026-11-07T10:00:00Z' } as Job);
});
afterEach(() => {
  unmount();
  document.body.innerHTML = '';
});

describe('a held row', () => {
  it('sets the other settings tiles trembling until it is let go', async () => {
    await mount(SettingsPage);
    const tiles = document.querySelector('.tiles')!;
    expect(tiles.classList).not.toContain('glim-drag-armed');
    drag(tiles.querySelector('[data-stab="general"]')!, 'dragstart');
    await nextTick();
    expect(tiles.classList).toContain('glim-drag-armed');
    expect(tiles.querySelector('[data-stab="general"]')!.classList).toContain('dragging');
    drag(tiles.querySelector('[data-stab="general"]')!, 'dragend');
    await nextTick();
    expect(tiles.classList).not.toContain('glim-drag-armed');
  });

  it('sets the other jobs trembling until it is let go', async () => {
    await mount(JobsBody);
    const list = document.querySelector('#jobs-body .job')!.parentElement!;
    expect(list.querySelectorAll(':scope > .job')).toHaveLength(3);
    drag(list.querySelector('[data-job="b"]')!, 'dragstart');
    await nextTick();
    expect(list.classList).toContain('glim-drag-armed');
    drag(list.querySelector('[data-job="b"]')!, 'dragend');
    await nextTick();
    expect(list.classList).not.toContain('glim-drag-armed');
  });
});

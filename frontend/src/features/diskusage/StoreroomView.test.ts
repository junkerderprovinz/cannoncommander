import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, h, nextTick } from 'vue';
import type { StoreroomScan, StoreroomStep } from '@/api/storeroom';
import { addMessages } from '@/i18n';
import de from './de.json';
import StoreroomView from './StoreroomView.vue';

const asked = vi.hoisted(() => ({ lines: [] as string[][], answer: true }));
vi.mock('./askSteps', () => ({ askSteps: async (_title: string, text: string[]) => { asked.lines.push(text); return asked.answer; } }));

let answer: StoreroomScan;
let calls: { path: string; body?: unknown }[];
const plan = (body: { op: string; refs: { kind?: string; id: string }[] }): StoreroomStep[] => body.refs.map((r) => {
  const f = answer.finds.find((x) => x.id === r.id), e = answer.room.find((x) => x.id === r.id);
  if (body.op === 'restore') return { kind: e!.kind, name: e!.name, from: e!.held, to: e!.from, forGood: false, done: false };
  return { kind: f!.kind, name: f!.name, from: f!.id, to: f!.kind === 'appdata' ? '/mnt/user/appdata/.storeroom/ab12/' + f!.name : undefined, forGood: f!.forGood, done: false };
});

beforeAll(() => addMessages('diskusage', { de }));
beforeEach(() => {
  setActivePinia(createPinia());
  asked.lines = [];
  asked.answer = true;
  calls = [];
  answer = {
    docker: true, write: true,
    finds: [
      { kind: 'template', id: '/boot/config/plugins/dockerMan/templates-user/my-alt.xml', name: 'alt', path: '/boot/config/plugins/dockerMan/templates-user/my-alt.xml', size: 2048, forGood: false },
      { kind: 'appdata', id: '/mnt/user/appdata/verwaist', name: 'verwaist', path: '/mnt/user/appdata/verwaist', size: 5 << 20, forGood: false },
      { kind: 'network', id: 'n5', name: 'alt-netz', size: 0, forGood: false },
      { kind: 'image', id: 'sha256:aabbccddeeff00112233', name: '', size: 100 << 20, forGood: true },
      { kind: 'cache', id: 'build-cache', name: '', size: 30 << 20, forGood: true },
    ],
    room: [{ id: 'e1', kind: 'appdata', name: 'frueher', from: '/mnt/user/appdata/frueher', held: '/mnt/user/appdata/.storeroom/e1/frueher', at: '2026-11-10T08:00:00Z', size: 1 << 20 }],
  };
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    const path = new URL(url, 'http://cc.test').pathname.replace('/api/v1', '');
    const body = init?.body ? JSON.parse(String(init.body)) : undefined;
    calls.push({ path, body });
    const out = path === '/storeroom/scan' ? answer : { steps: plan(body).map((s) => ({ ...s, done: path === '/storeroom/run' })) };
    return new Response(JSON.stringify(out), { status: 200, headers: { 'Content-Type': 'application/json' } });
  }));
});
let app: ReturnType<typeof createApp> | undefined;
afterEach(() => {
  app?.unmount();
  document.body.innerHTML = '';
});

const settle = async () => {
  for (let i = 0; i < 8; i++) {
    await new Promise((r) => setTimeout(r, 0));
    await nextTick();
  }
};
async function mount() {
  const summary: string[] = [];
  app = createApp({ render: () => h(StoreroomView, { onSummary: (s: string) => summary.push(s) }) });
  app.mount(document.body.appendChild(document.createElement('div')));
  await settle();
  return summary;
}
const tick = async (sel: string) => {
  document.querySelector<HTMLElement>(`${sel} .glim-checkbox, ${sel} input, ${sel} [role="checkbox"]`)!.click();
  await settle();
};
const press = async (sel: string) => {
  document.querySelector<HTMLElement>(sel)!.click();
  await settle();
};
const sections = () => [...document.querySelectorAll<HTMLElement>('[data-section]')].map((s) => s.dataset.section);

describe('the storeroom view', () => {
  it('lists what nothing uses by kind, then what waits in the storeroom', async () => {
    const summary = await mount();
    expect(sections()).toEqual(['template', 'appdata', 'network', 'image', 'room']);
    const images = [...document.querySelectorAll('[data-section="image"] .du-room-row .n')].map((x) => x.textContent);
    expect(images).toEqual(['Image ohne Tag aabbccddeeff', 'Build-Cache']);
    expect(document.querySelector('[data-entry="e1"]')?.textContent).toContain('Abgestellt am 10.11.2026');
    expect(summary.at(-1)).toBe('5 zum Wegräumen, 1 in der Abstellkammer');
  });

  it('shows the plan and clears away only once it is agreed to', async () => {
    await mount();
    await tick('[data-find="appdata|/mnt/user/appdata/verwaist"]');
    await press('[data-store="appdata"]');
    expect(asked.lines).toEqual([['verwaist kommt in die Abstellkammer nach /mnt/user/appdata/.storeroom/ab12/verwaist']]);
    expect(calls.map((c) => c.path)).toEqual(['/storeroom/scan', '/storeroom/plan', '/storeroom/run', '/storeroom/scan']);
    expect(calls[2]!.body).toEqual({ op: 'store', refs: [{ kind: 'appdata', id: '/mnt/user/appdata/verwaist' }] });
  });

  it('does nothing when the question is left', async () => {
    asked.answer = false;
    await mount();
    await tick('[data-find="network|n5"]');
    await press('[data-store="network"]');
    expect(calls.map((c) => c.path)).toEqual(['/storeroom/scan', '/storeroom/plan', '/storeroom/scan']);
  });

  it('says that images and the build cache go for good', async () => {
    await mount();
    await tick('[data-find="image|sha256:aabbccddeeff00112233"]');
    await tick('[data-find="cache|build-cache"]');
    await press('[data-store="image"]');
    expect(asked.lines[0]).toEqual(['Image ohne Tag aabbccddeeff wird endgültig entfernt', 'Build-Cache wird endgültig entfernt']);
  });

  it('brings an entry back to where it came from', async () => {
    await mount();
    await tick('[data-entry="e1"]');
    await press('[data-restore]');
    expect(asked.lines).toEqual([['frueher kommt zurück nach /mnt/user/appdata/frueher']]);
    expect(calls[2]!.body).toEqual({ op: 'restore', refs: [{ id: 'e1' }] });
  });

  it('only looks while write access is off', async () => {
    answer.write = false;
    await mount();
    expect(document.querySelector('.du-room-note')?.textContent).toBe(de.storeroom.readOnly);
    expect(document.querySelector<HTMLButtonElement>('[data-store="appdata"]')!.disabled).toBe(true);
  });

  it('lists nothing and says what is missing without docker', async () => {
    answer = { docker: false, write: true, finds: [], room: [] };
    await mount();
    expect(sections()).toEqual(['room']);
    expect(document.querySelector('.du-room-note')?.textContent).toBe(de.storeroom.noDocker);
  });
});

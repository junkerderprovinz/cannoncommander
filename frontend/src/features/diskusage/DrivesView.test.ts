import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, h, nextTick } from 'vue';
import type { UnraidDrive, UnraidDrives } from '@/api/unraid';
import { DAY, GB, setClock, TB } from '@/core/format';
import { addMessages } from '@/i18n';
import de from './de.json';
import DrivesView from './DrivesView.vue';

const NOW = Date.parse('2026-11-17T09:30:00Z');
const today = Math.floor(NOW / DAY);
const grow = (from: number, perDay: number, days = 84) => Array.from({ length: days }, (_, i) => [today - days + 1 + i, Math.round(from + perDay * i)] as [number, number]);
const disk = (name: string, more: Partial<UnraidDrive> = {}): UnraidDrive => ({
  name, kind: 'data', status: 'DISK_OK', fsType: 'xfs', rotational: true, transport: 'ata', size: 4 * TB, fsSize: 4 * TB, used: 3 * TB, asleep: false, temp: 36,
  history: grow(3 * TB - 84 * 10 * GB, 10 * GB), ...more,
});
let answer: UnraidDrives;
let asked: string[] = [];

beforeAll(() => {
  addMessages('diskusage', { de });
  setClock(() => NOW);
});
beforeEach(() => {
  setActivePinia(createPinia());
  asked = [];
  answer = {
    pools: [disk('cache', { kind: 'pool', fsType: 'zfs', profile: 'mirror', rotational: false, transport: 'nvme', size: TB, fsSize: TB, used: TB / 2, temp: 41, fullIn: {},
      history: grow(TB / 2, 0), zfs: { fragmentation: 14, compressRatio: 1.41, scrub: { at: '2026-11-02T00:38:03Z', errors: 0, seconds: 842 } },
      members: [disk('cache', { kind: 'pool', device: 'nvme0n1', temp: 41, history: [] }), disk('cache2', { kind: 'pool', device: 'nvme1n1', temp: 43, history: [] })] })],
    array: [
      disk('disk1', { asleep: true, asleepSince: '2026-11-17T02:14:00Z', temp: 33, tempAt: '2026-11-17T02:10:00Z', fullIn: { 4: 9.6, 8: 12.2, 12: 11.8 } }),
      disk('disk2', { fullIn: { 12: 30 } }),
      disk('parity', { kind: 'parity', fsSize: 0, used: 0, history: [], temp: 34 }),
    ],
    parity: { at: '2026-10-10T05:47:54Z', seconds: 89155, errors: 0, exit: 0 },
  };
  vi.stubGlobal('fetch', vi.fn(async (url: string) => {
    asked.push(new URL(url, 'http://cc.test').pathname.replace('/api/v1', ''));
    return new Response(JSON.stringify(answer), { status: 200, headers: { 'Content-Type': 'application/json' } });
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
  app = createApp({ render: () => h(DrivesView) });
  app.mount(document.body.appendChild(document.createElement('div')));
  await settle();
}
const row = (name: string) => document.querySelector<HTMLElement>(`[data-drive="${name}"]`)!;
const pick = async (name: string) => {
  row(name).click();
  await settle();
};
const range = async (weeks: number) => {
  document.querySelector<HTMLElement>(`#du-range [data-sel-id="${weeks}"]`)!.click();
  await settle();
};
const outlook = () => document.querySelector('.du-outlook')?.textContent?.trim();

describe('the drives view', () => {
  it('lists the pool with its devices, then the array disks and parity', async () => {
    await mount();
    const names = [...document.querySelectorAll('.du-drow .du-dname .n')].map((x) => x.textContent?.trim());
    expect(names).toEqual(['Cache', 'nvme0n1', 'nvme1n1', 'Disk 1', 'Disk 2', 'Parität']);
    expect(document.querySelector('.glim-subcard .glim-info-icon')?.getAttribute('aria-label')).toBe(de.drives.hint);
  });

  it('shows a sleeping disk with a moon, its last values greyed out and a dashed sparkline', async () => {
    await mount();
    const r = row('disk1');
    expect(r.classList).toContain('asleep');
    expect(r.querySelector('.u-moon')?.getAttribute('data-tip')).toMatch(/^Disk 1 schläft seit \d\d:\d\d\./);
    const temp = r.querySelector('.du-temp')!;
    expect(temp.classList).toContain('stale');
    expect(temp.textContent).toContain('33');
    expect(temp.getAttribute('data-tip')).toMatch(/^Zuletzt gemessen um \d\d:\d\d$/);
    expect(r.querySelector('.du-spark path:not(.area)')).not.toBeNull();
    expect(row('disk2').classList).not.toContain('asleep');
    expect(row('disk2').querySelector('.u-moon')).toBeNull();
  });

  it('asks the server for nothing but what it already knows', async () => {
    await mount();
    await pick('disk1');
    await range(12);
    expect(new Set(asked)).toEqual(new Set(['/unraid/drives']));
  });

  it('turns the meter red from 95 percent on', async () => {
    answer.array[1]!.used = 0.96 * 4 * TB;
    await mount();
    expect(row('disk2').querySelector('.glim-meter-track--full')).not.toBeNull();
    expect(row('disk1').querySelector('.glim-meter-track--full')).toBeNull();
  });

  it('says when a growing disk is full, for the weeks shown', async () => {
    await mount();
    await pick('disk1');
    expect(outlook()).toBe('Voll in etwa 12 Wochen');
    await range(4);
    expect(outlook()).toBe('Voll in etwa 10 Wochen');
  });

  it('says nothing about full while the trend is flat or falling', async () => {
    await mount();
    await pick('cache');
    expect(outlook()).toBeUndefined();
    await pick('disk2');
    expect(outlook()).toBeUndefined();
    await range(12);
    expect(outlook()).toBe('Voll in etwa 30 Wochen');
  });

  it('shows the change over the weeks shown and the details of the picked drive', async () => {
    await mount();
    await pick('disk2');
    expect(document.querySelector('.du-chart-foot .glim-num')?.textContent).toMatch(/^\+.*in 8 Wochen$/);
    const facts = [...document.querySelectorAll('.du-facts .glim-field-row')].map((x) => [x.querySelector('.glim-field-row-label')?.textContent?.trim(), x.querySelector('.glim-pill')?.textContent?.trim()]);
    expect(facts[0]).toEqual(['Zustand', 'Läuft']);
    expect(facts[1]).toEqual(['Letzte Paritätsprüfung', '10.10.2026, 0 Fehler']);
    await pick('cache');
    const pool = [...document.querySelectorAll('.du-facts .glim-field-row-label')].map((x) => x.textContent?.trim());
    expect(pool).toEqual(['Zustand', 'Letzter Scrub', 'Fragmentierung', 'Kompression', 'Dateisystem']);
  });
});

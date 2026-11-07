import { createPinia, getActivePinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, h } from 'vue';
import { unraid, type UnraidShare } from '@/api/unraid';
import { emit } from '@/api/sse';
import { toasts } from '@/core/toast';
import { addMessages } from '@/i18n';
import de from './de.json';
import en from './en.json';
import { useUnraid } from './state';
import { moverPool, moverWhen, shareBadge } from './store';
import WhereWell from './WhereWell.vue';

addMessages('unraid', { de, en } as never);

const share = (s: Partial<UnraidShare>): UnraidShare => ({ name: 'downloads', primary: { type: 'pool', name: 'cache' }, allocator: 'highwater', disks: ['disk1'], free: 0, used: 0, ...s });
const downloads = share({ secondary: { type: 'array' }, mover: 'toSecondary' });

beforeEach(() => {
  setActivePinia(createPinia());
  toasts.splice(0);
});

describe('mover schedule', () => {
  it('reads a daily run as a time of day', () => {
    expect(moverWhen('40 3 * * *')).toBe('um 3:40 Uhr');
    expect(moverWhen('5 22 * * *')).toBe('um 22:05 Uhr');
  });
  it('reads runs every few hours', () => {
    expect(moverWhen('0 */2 * * *')).toBe('alle 2 Stunden');
    expect(moverWhen('15 * * * *')).toBe('jede Stunde');
  });
  it('falls back to the next run for anything rarer', () => {
    expect(moverWhen('40 3 * * 1')).toBe('beim nächsten Lauf');
    expect(moverWhen(undefined)).toBe('beim nächsten Lauf');
  });
});

describe('share badge', () => {
  it('names the way the mover goes and what still waits on the cache', () => {
    const b = shareBadge(downloads, '40 3 * * *', { files: 19, size: 4.3 * 1024 ** 3 });
    expect(b.label).toBe('Cache → Array');
    expect(b.glyph).toBe('IconBolt');
    expect(b.tip).toBe('Share downloads: Primärspeicher Cache, Sekundärspeicher Array. Neue Dateien landen auf dem Cache, der Mover schiebt sie um 3:40 Uhr auf Disk 1. Hier liegen noch 19 Dateien mit 4,3 GB auf dem Cache.');
  });
  it('turns the arrow round for a share the mover fills from the array', () => {
    const b = shareBadge(share({ name: 'appdata', secondary: { type: 'array' }, mover: 'toPrimary' }));
    expect(b.label).toBe('Array → Cache');
    expect(b.tip).toContain('der Mover holt Reste vom Array (Disk 1) dorthin.');
  });
  it('says where files go without a second storage', () => {
    expect(shareBadge(share({ name: 'filme', primary: { type: 'array' }, disks: ['disk1', 'disk2', 'disk3'] })))
      .toEqual({ label: 'nur Array', glyph: 'IconDatabase', tip: 'Share filme: Primärspeicher Array, kein Sekundärspeicher. Neue Dateien landen direkt auf Disk 1, Disk 2, Disk 3.' });
    expect(shareBadge(share({ name: 'system', disks: [] })).tip).toContain('Alles bleibt auf dem Cache.');
  });
  it('only counts a pool the mover still empties', () => {
    expect(moverPool(downloads)).toBe('cache');
    expect(moverPool(share({ mover: 'toPrimary', secondary: { type: 'array' } }))).toBe('');
    expect(moverPool(share({ primary: { type: 'pool', name: 'fastcache' }, secondary: { type: 'array' }, mover: 'toSecondary' }))).toBe('fastcache');
  });
});

describe('unraid store', () => {
  const state = () => {
    const U = useUnraid();
    U.st = {
      available: true, write: true, spindownDelay: 45, shares: [downloads],
      parity: { running: true, paused: false, kind: 'check', pos: 0, size: 1, percent: 40, lastErrors: 0, lastExit: 0 },
      disks: [
        { name: 'disk1', kind: 'data', mounted: true, size: 1, free: 1, spundown: true, rotational: true },
        { name: 'disk2', kind: 'data', mounted: true, size: 1, free: 1, spundown: false, rotational: true },
        { name: 'cache', kind: 'pool', mounted: true, size: 1, free: 1, spundown: false, rotational: false },
      ],
    };
    return U;
  };

  it('knows a path on the array and nothing elsewhere', () => {
    const U = state();
    expect(U.place('/mnt/user/downloads/a.iso')).toEqual({ vol: 'user', share: 'downloads', rel: 'downloads/a.iso' });
    expect(U.place('/mnt/cache/downloads')).toEqual({ vol: 'cache', share: 'downloads', rel: 'downloads' });
    expect(U.place('/remotes/nas')).toBeNull();
    expect(U.place('/mnt/user/a.zip::/x')).toBeNull();
  });

  it('warns once before a sleeping disk spins up and then counts it awake', () => {
    const U = state();
    U.wake(['disk1', 'disk2', 'disk1']);
    expect(toasts.map((x) => x.text)).toEqual(['Disk 1 schläft und fährt jetzt hoch, das dauert ein paar Sekunden']);
    expect(U.asleep('disk1')).toBe(false);
    U.wake(['disk1']);
    expect(toasts).toHaveLength(1);
  });

  it('follows the spin and parity events', () => {
    const U = state();
    emit({ type: 'unraid.spin', data: { asleep: ['disk2'], changed: ['disk1', 'disk2'] } });
    expect([U.asleep('disk1'), U.asleep('disk2')]).toEqual([false, true]);
    emit({ type: 'unraid.parity', data: { running: false, paused: false, pos: 0, size: 1, percent: 0, lastErrors: 0, lastExit: 0 } });
    expect(U.st!.parity.running).toBe(false);
    expect(toasts.at(-1)!.text).toBe('Die Paritätsprüfung ist fertig, wartende Aufgaben laufen jetzt weiter');
  });

  it('shows in Eigenschaften where a file lies and its other names', async () => {
    state();
    vi.spyOn(unraid, 'where').mockResolvedValue({
      path: '/mnt/user/downloads/a.mkv', dir: false, files: 1, size: 5,
      on: [{ device: 'disk1', files: 1, size: 5, asleep: true }],
      links: { count: 2, paths: ['/mnt/user/filme/a.mkv'], complete: true },
    });
    const host = document.createElement('div');
    createApp({ render: () => h(WhereWell, { path: '/mnt/user/downloads/a.mkv' }) }).use(getActivePinia()!).mount(host);
    await vi.waitFor(() => expect(host.querySelector('.u-where')).not.toBeNull());
    expect(host.querySelector('.u-pills')!.textContent).toBe('Disk 1');
    expect([...host.querySelectorAll('.u-links .bk-path')].map((p) => p.textContent)).toEqual(['/mnt/user/filme/a.mkv']);
  });

  it('keeps quiet about a parity run that was cancelled', () => {
    const U = state();
    emit({ type: 'unraid.parity', data: { running: false, paused: false, pos: 0, size: 1, percent: 0, lastErrors: 0, lastExit: -4 } });
    expect(U.st!.parity.running).toBe(false);
    expect(toasts).toHaveLength(0);
  });
});

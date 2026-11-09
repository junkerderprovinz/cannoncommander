import { createPinia, setActivePinia } from 'pinia';
import { beforeAll, describe, expect, it } from 'vitest';
import { nextTick } from 'vue';
import { action, actions } from '@/core/actions';
import { resolveItems } from '@/core/menu';
import { useUser } from '@/stores/user';
import gather from '@/features/gather';
import { lines } from '@/features/palette/rows';
import { rowOf } from '@/features/shellmenu/custom';
import unraid from '.';
import { useUnraid } from './state';

const ONLY_UNRAID = ['cacheOnly', 'fixPerms', 'spinDown', 'gather'];

beforeAll(() => {
  setActivePinia(createPinia());
  unraid();
  gather();
  useUser().me = { name: 'jdp', full: '', role: 'admin', home: '/home/jdp' };
  // The neighbours of Rechte wie Unraid setzen in the context menu.
  for (const id of ['props', 'copyPath']) action(id, { label: id, run: () => {} });
});

const ids = (items: ReturnType<typeof resolveItems>) => items.map((it) => (typeof it === 'object' && 'id' in it ? it.id : it));

describe('what only Unraid can do', () => {
  it('is not there at all where there is no Unraid, not even greyed out', async () => {
    useUnraid().st = null;
    await nextTick();
    for (const id of ONLY_UNRAID) {
      expect(actions[id], id).toBeUndefined();
      expect(rowOf(id), id).toBeNull();
    }
    expect(ids(resolveItems(['props', 'fixPerms', 'copyPath']))).toEqual(['props', 'copyPath']);
    expect(lines('Unraid').some((l) => 'a' in l && ONLY_UNRAID.includes(l.a!.id))).toBe(false);
  });

  it('is offered to an admin on Unraid', async () => {
    useUnraid().st = { available: true, write: true, spindownDelay: 45, shares: [], disks: [] } as never;
    await nextTick();
    for (const id of ['cacheOnly', 'fixPerms', 'gather']) expect(actions[id], id).toBeDefined();
    expect(actions.fixPerms!.when!()).toBe(true);
    expect(rowOf('fixPerms')).not.toBeNull();
  });
});

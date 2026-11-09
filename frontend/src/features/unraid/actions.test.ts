import { createPinia, setActivePinia } from 'pinia';
import { beforeAll, describe, expect, it } from 'vitest';
import { actions } from '@/core/actions';
import { useUser } from '@/stores/user';
import gather from '@/features/gather';
import unraid from '.';
import { useUnraid } from './state';

beforeAll(() => {
  setActivePinia(createPinia());
  unraid();
  gather();
  useUser().me = { name: 'jdp', full: '', role: 'admin', home: '/home/jdp' };
});

describe('what only Unraid can do', () => {
  it('stays out of the menus where there is no Unraid, for an admin too', () => {
    useUnraid().st = null;
    expect(actions.fixPerms!.when!()).toBe(false);
    expect(actions.gather!.when!()).toBe(false);
  });

  it('is offered to an admin on Unraid', () => {
    useUnraid().st = { available: true, write: true, spindownDelay: 45, shares: [], disks: [] } as never;
    expect(actions.fixPerms!.when!()).toBe(true);
    expect(actions.gather!.when!()).toBe(true);
  });
});

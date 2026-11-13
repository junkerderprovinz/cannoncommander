import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it } from 'vitest';
import { actions } from '@/core/actions';
import { useUser } from '@/stores/user';
import { installSignOut } from '.';

beforeEach(() => {
  setActivePinia(createPinia());
  installSignOut();
});

describe('sign out', () => {
  it('stays away while nobody has to sign in', () => {
    useUser().me = { name: 'admin', full: '', role: 'admin', signIn: false };
    expect(actions.logout.when?.()).toBe(false);
  });

  it('shows once signing in is on', () => {
    useUser().me = { name: 'admin', full: '', role: 'admin', signIn: true };
    expect(actions.logout.when?.()).toBe(true);
  });
});

import { createPinia, setActivePinia } from 'pinia';
import { beforeAll, describe, expect, it } from 'vitest';
import { settingsTabs } from '@/core/views';
import setup from '.';

beforeAll(() => {
  setActivePinia(createPinia());
  setup();
});

describe('the settings', () => {
  it('open on the account and its security', () => {
    expect(settingsTabs[0]?.id).toBe('security');
    expect(settingsTabs.filter((t) => t.id === 'security')).toHaveLength(1);
  });
});

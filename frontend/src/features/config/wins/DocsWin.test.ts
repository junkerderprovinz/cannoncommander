import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { createApp, h, nextTick } from 'vue';
import type { WinEntry } from '@/core/windows';
import { DOCS } from '@/features/config/support';
import { addMessages, setLanguage } from '@/i18n';
import en from '../en.json';
import DocsWin from './DocsWin.vue';

beforeAll(() => {
  addMessages('config', { en });
  setLanguage('en');
});

let app: ReturnType<typeof createApp> | undefined;
afterEach(() => {
  app?.unmount();
  vi.restoreAllMocks();
  document.body.innerHTML = '';
});

async function open() {
  const win = { foot: [] } as unknown as WinEntry;
  app = createApp({ render: () => h(DocsWin, { win }) });
  app.mount(document.body.appendChild(document.createElement('div')));
  await nextTick();
  return win;
}

describe('the handbook', () => {
  it('opens the documentation site in a new tab and stays open', async () => {
    const opened = vi.spyOn(window, 'open').mockReturnValue(null);
    const win = await open();
    const site = win.foot.find((b) => b.g === 'IconExternalLink');
    expect(site?.label).toBe('Open the documentation');
    expect(site?.run?.(document.body)).toBe(false);
    expect(opened).toHaveBeenCalledWith(DOCS, '_blank', 'noopener');
  });
});

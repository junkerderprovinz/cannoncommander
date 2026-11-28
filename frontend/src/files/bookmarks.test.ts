// @vitest-environment jsdom
import { expect, it, vi } from 'vitest';

const { toasts } = vi.hoisted(() => ({ toasts: [] as string[] }));
vi.mock('@/core/toast', () => ({ toast: (text: string) => toasts.push(text) }));

const { copyInternalLink } = await import('./bookmarks');

it('copies the internal link on a page opened over plain http, where the browser has no clipboard api', async () => {
  let copied = '';
  Object.defineProperty(window, 'isSecureContext', { value: false, configurable: true });
  Object.defineProperty(navigator, 'clipboard', { value: undefined, configurable: true });
  document.execCommand = vi.fn(() => {
    copied = document.body.querySelector<HTMLTextAreaElement>(':scope > textarea')?.value ?? '';
    return true;
  });
  copyInternalLink('/mnt/user/Fotos 2026');
  await vi.waitFor(() => expect(toasts).toHaveLength(1));
  expect(copied).toBe(location.href.split('#')[0] + '#/mnt/user/Fotos%202026');
});

it('says nothing was copied when the browser refuses', async () => {
  toasts.length = 0;
  Object.defineProperty(navigator, 'clipboard', { value: undefined, configurable: true });
  document.execCommand = vi.fn(() => false);
  copyInternalLink('/mnt/user');
  await new Promise((done) => setTimeout(done));
  expect(toasts).toEqual([]);
});

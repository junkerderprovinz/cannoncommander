import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, h, nextTick } from 'vue';
import { addTo } from '@/core/extend';
import type { Entry } from './entries';
import EntryGlyph from './EntryGlyph.vue';

// jsdom loads no pictures, so each one here waits until the test says it arrived or failed.
const pictures: { src: string; onload: (() => void) | null; onerror: (() => void) | null }[] = [];
class FakeImage {
  onload: (() => void) | null = null;
  onerror: (() => void) | null = null;
  set src(v: string) { pictures.push({ src: v, onload: () => this.onload?.(), onerror: () => this.onerror?.() }); }
}

const apps: ReturnType<typeof createApp>[] = [];
let drop: (() => void) | undefined;
function show(name: string) {
  const app = createApp({ render: () => h(EntryGlyph, { k: { name } as Entry, folder: '/C', px: 16 }) });
  const host = document.body.appendChild(document.createElement('div'));
  app.mount(host);
  apps.push(app);
  return host;
}

beforeEach(() => {
  pictures.length = 0;
  vi.stubGlobal('Image', FakeImage);
  drop = addTo('entryIcons', (k) => (k.name.endsWith('.exe') ? `/icon/${k.name}` : null));
});
afterEach(() => {
  apps.splice(0).forEach((a) => a.unmount());
  drop?.();
  document.body.innerHTML = '';
  vi.unstubAllGlobals();
});

describe('an entry glyph', () => {
  it('shows the glyph until the picture has arrived', async () => {
    const host = show('setup.exe');
    await nextTick();
    expect(host.querySelector('svg')).not.toBeNull();
    expect(host.querySelector('img')).toBeNull();
    pictures.find((p) => p.src === '/icon/setup.exe')!.onload!();
    await nextTick();
    expect(host.querySelector('img')?.getAttribute('src')).toBe('/icon/setup.exe');
  });

  it('keeps the glyph when the picture never comes', async () => {
    const host = show('broken.exe');
    await nextTick();
    pictures.find((p) => p.src === '/icon/broken.exe')!.onerror!();
    await nextTick();
    expect(host.querySelector('svg')).not.toBeNull();
    expect(host.querySelector('img')).toBeNull();
  });

  it('asks once for a picture many rows share', async () => {
    show('tool.exe');
    show('tool.exe');
    await nextTick();
    expect(pictures.filter((p) => p.src === '/icon/tool.exe')).toHaveLength(1);
  });

  it('asks for nothing without a picture', async () => {
    const host = show('notes.txt');
    await nextTick();
    expect(pictures).toHaveLength(0);
    expect(host.querySelector('svg')).not.toBeNull();
  });
});

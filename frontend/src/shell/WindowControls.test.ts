import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import { host } from '@/core/host';
import { addMessages, setLanguage } from '@/i18n';
import { drawsOwnFrame, edgeAt } from './frame';
import WindowControls from './WindowControls.vue';

beforeAll(() => {
  setLanguage('de');
  addMessages('desktop', { de: { minimise: 'Minimieren', maximise: 'Maximieren', restore: 'Wiederherstellen' } });
  addMessages('core', { de: { close: 'Schließen' } });
});

describe('the window frame', () => {
  afterEach(() => { Object.assign(host, { desktop: false, os: '' }); });

  it('is drawn by the page only in the desktop app on Windows and Linux', () => {
    for (const [desktop, os, drawn] of [[false, 'windows', false], [true, 'windows', true], [true, 'linux', true], [true, 'darwin', false]] as const) {
      Object.assign(host, { desktop, os });
      expect(drawsOwnFrame(), `${desktop} ${os}`).toBe(drawn);
    }
  });

  it('grabs the side or the corner under the pointer and nothing inside', () => {
    expect(edgeAt(500, 300, 1000, 600)).toBe('');
    expect(edgeAt(2, 300, 1000, 600)).toBe('w');
    expect(edgeAt(998, 300, 1000, 600)).toBe('e');
    expect(edgeAt(500, 1, 1000, 600)).toBe('n');
    expect(edgeAt(500, 598, 1000, 600)).toBe('s');
    expect(edgeAt(990, 598, 1000, 600)).toBe('se');
    expect(edgeAt(2, 10, 1000, 600)).toBe('nw');
    expect(edgeAt(10, 10, 1000, 600)).toBe('');
  });
});

describe("the window's own buttons at the end of the head", () => {
  let maximised = false;
  let calls: string[] = [];
  let posted: string[] = [];
  let app: ReturnType<typeof createApp>;

  const settle = async () => {
    await new Promise((done) => setTimeout(done));
    await nextTick();
  };
  const head = () => document.querySelector<HTMLElement>('.head')!;
  const button = (name: string) => document.querySelector<HTMLButtonElement>(`.glim-window-button[aria-label="${name}"]`)!;
  const mouse = (el: EventTarget, type: string, init: MouseEventInit = {}) => el.dispatchEvent(new MouseEvent(type, { bubbles: true, cancelable: true, ...init }));

  beforeEach(async () => {
    maximised = false;
    calls = [];
    posted = [];
    vi.stubGlobal('fetch', vi.fn(async (url: string, init: RequestInit = {}) => {
      const method = init.method ?? 'GET';
      calls.push(`${method} ${url}${init.body ? ' ' + String(init.body) : ''}`);
      if (method === 'GET') return Response.json({ maximised });
      return new Response(null, { status: 204 });
    }));
    vi.stubGlobal('chrome', { webview: { postMessage: (m: string) => posted.push(m) } });
    document.body.innerHTML = '<div class="app"><header class="head"><button class="settings">Einstellungen</button><span id="controls"></span></header></div>';
    app = createApp(WindowControls);
    app.mount('#controls');
    await settle();
  });
  afterEach(() => {
    app.unmount();
    document.body.innerHTML = '';
    vi.unstubAllGlobals();
  });

  it('works the window from its three buttons', async () => {
    button('Minimieren').click();
    button('Maximieren').click();
    button('Schließen').click();
    await settle();
    expect(calls).toEqual([
      'GET /api/v1/window',
      'POST /api/v1/window/minimise',
      'PUT /api/v1/window {"maximised":true}',
      'POST /api/v1/window/close',
    ]);
  });

  it('offers to restore a maximised window once its size has settled', async () => {
    maximised = true;
    vi.useFakeTimers();
    window.dispatchEvent(new Event('resize'));
    vi.advanceTimersByTime(150);
    vi.useRealTimers();
    await settle();
    button('Wiederherstellen').click();
    await settle();
    expect(calls.at(-1)).toBe('PUT /api/v1/window {"maximised":false}');
  });

  it('moves the window once a press on the head moves, and keeps a click a click', () => {
    mouse(head(), 'mousedown', { button: 0 });
    mouse(head(), 'mouseup');
    mouse(head(), 'mousemove', { buttons: 1 });
    expect(posted).toEqual([]);
    mouse(head(), 'mousedown', { button: 0 });
    mouse(head(), 'mousemove', { buttons: 1 });
    expect(posted).toEqual(['wails:drag']);
  });

  it('moves the window from the edge of the app around the head as well', () => {
    mouse(document.querySelector('.app')!, 'mousedown', { button: 0 });
    mouse(document.querySelector('.app')!, 'mousemove', { buttons: 1 });
    expect(posted).toEqual(['wails:drag']);
  });

  it('leaves a press on anything in the head to that thing', () => {
    mouse(document.querySelector('.settings')!, 'mousedown', { button: 0 });
    mouse(document.querySelector('.settings')!, 'mousemove', { buttons: 1 });
    mouse(button('Minimieren'), 'mousedown', { button: 0 });
    mouse(button('Minimieren'), 'mousemove', { buttons: 1 });
    expect(posted).toEqual([]);
  });

  it('maximises on a double click on the head but not on a button', async () => {
    mouse(button('Minimieren'), 'dblclick');
    mouse(document.querySelector('.settings')!, 'dblclick');
    mouse(head(), 'dblclick');
    await settle();
    expect(calls.filter((c) => c.startsWith('PUT'))).toEqual(['PUT /api/v1/window {"maximised":true}']);
  });

  it('resizes from an edge until the window is maximised', async () => {
    mouse(window, 'pointermove', { clientX: innerWidth - 1, clientY: 300 });
    mouse(window, 'pointerdown', { clientX: innerWidth - 1, clientY: 300, button: 0 });
    expect(posted).toEqual(['wails:resize:e-resize']);
    expect(document.body.style.cursor).toBe('ew-resize');

    maximised = true;
    vi.useFakeTimers();
    window.dispatchEvent(new Event('resize'));
    vi.advanceTimersByTime(150);
    vi.useRealTimers();
    await settle();
    mouse(window, 'pointermove', { clientX: innerWidth - 1, clientY: 200 });
    mouse(window, 'pointerdown', { clientX: innerWidth - 1, clientY: 200, button: 0 });
    expect(posted).toEqual(['wails:resize:e-resize']);
    expect(document.body.style.cursor).toBe('');
  });
});

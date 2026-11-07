import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import { toasts } from '@/core/toast';
import { winStack } from '@/core/windows';
import { addMessages, setLanguage } from '@/i18n';
import { useUser } from '@/stores/user';
import { FakeSocket } from '@/testing/fakeSocket';
import de from './de.json';
import en from './en.json';
import { terminalOn } from './state';
import TerminalBody from './TerminalBody.vue';
import { askTerminal } from './index';

addMessages('terminal', { de, en });

// xterm draws on a canvas jsdom does not have, so a stand-in keeps what it is told.
const { FakeTerminal, screens } = vi.hoisted(() => {
  const screens: InstanceType<typeof FakeTerminal>[] = [];
  class FakeTerminal {
    written: (string | Uint8Array)[] = [];
    disposed = false;
    cols = 80;
    rows = 24;
    options: Record<string, unknown> = {};
    typed: ((d: string) => void) | null = null;
    constructor() { screens.push(this); }
    loadAddon() {}
    open() {}
    focus() {}
    write(d: string | Uint8Array) { this.written.push(d); }
    reset() {}
    onData(fn: (d: string) => void) { this.typed = fn; return { dispose() {} }; }
    onResize() { return { dispose() {} }; }
    attachCustomKeyEventHandler() {}
    getSelection() { return ''; }
    dispose() { this.disposed = true; }
  }
  return { FakeTerminal, screens };
});
vi.mock('@xterm/xterm', () => ({ Terminal: FakeTerminal }));
vi.mock('@xterm/addon-fit', () => ({ FitAddon: class { fit() {} } }));
vi.mock('@xterm/xterm/css/xterm.css', () => ({}));

let feature = false;
let asked: string[] = [];
function server(url: string) {
  asked.push(url);
  if (url.startsWith('/api/v1/system')) return Response.json({ features: { terminal: feature }, started: '2026-01-01T00:00:00Z' });
  if (url.startsWith('/api/v1/terminal')) return new Response(null, { status: 204 });
  return new Response(null, { status: 404 });
}

beforeEach(() => {
  setActivePinia(createPinia());
  setLanguage('de');
  toasts.splice(0);
  winStack.splice(0);
  screens.splice(0);
  FakeSocket.all = [];
  asked = [];
  feature = false;
  terminalOn.value = false;
  vi.stubGlobal('fetch', vi.fn(async (url: string) => server(url)));
  vi.stubGlobal('WebSocket', FakeSocket);
  vi.stubGlobal('ResizeObserver', class { observe() {} disconnect() {} });
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null);
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

const as = (role: string) => { useUser().me = { name: 'jdp', full: 'jdp', role }; };

describe('askTerminal', () => {
  it('tells someone who is no admin that it is for admins', async () => {
    as('user');
    await askTerminal('/mnt/user');
    expect(toasts.map((x) => x.text)).toEqual(['Das Terminal können nur Admins öffnen.']);
    expect(winStack).toHaveLength(0);
    expect(asked).toEqual([]);
  });

  it('tells an admin it is off and leads to the switch', async () => {
    as('admin');
    await askTerminal('/mnt/user');
    expect(toasts).toHaveLength(1);
    expect(toasts[0]!.text).toBe('Das Terminal ist noch ausgeschaltet. Du kannst es in den Einstellungen unter Integrationen einschalten.');
    expect(toasts[0]!.act?.label).toBe('Zu den Einstellungen');
    expect(winStack).toHaveLength(0);
  });

  it('looks again before saying it is off, in case it was switched on meanwhile', async () => {
    as('admin');
    feature = true;
    await askTerminal('/mnt/user/appdata');
    expect(toasts).toHaveLength(0);
    expect(asked).toContain('/api/v1/terminal?path=%2Fmnt%2Fuser%2Fappdata');
    expect(winStack).toHaveLength(1);
    expect(winStack[0]!.title).toBe('Terminal');
  });

  it('opens one window per terminal', async () => {
    as('admin');
    terminalOn.value = true;
    await askTerminal('/mnt/user');
    await askTerminal('/mnt/user/isos');
    expect(winStack.map((w) => w.props?.path)).toEqual(['/mnt/user', '/mnt/user/isos']);
  });
});

describe('the terminal window', () => {
  function mount() {
    const el = document.createElement('div');
    document.body.append(el);
    const close = vi.fn();
    const app = createApp(TerminalBody, { path: '/mnt/user', close });
    app.mount(el);
    return { app, close, ws: () => FakeSocket.all.at(-1)!, screen: () => screens.at(-1)! };
  }

  it('relays the shell both ways', async () => {
    const t = mount();
    t.ws().open();
    t.ws().onmessage?.({ data: new TextEncoder().encode('hallo').buffer });
    t.screen().typed?.('ls\r');
    expect(new TextDecoder().decode(t.screen().written.at(-1) as Uint8Array)).toBe('hallo');
    expect(new TextDecoder().decode(t.ws().sent[0] as Uint8Array)).toBe('ls\r');
    t.app.unmount();
  });

  it('closes its window when the shell exits', () => {
    const t = mount();
    t.ws().open();
    t.ws().serverCloses(1000, 'exited');
    expect(t.close).toHaveBeenCalled();
    t.app.unmount();
  });

  it('shows a dropped connection and starts again on request', async () => {
    const t = mount();
    t.ws().open();
    t.ws().serverCloses(1006);
    await nextTick();
    expect(document.body.textContent).toContain('Die Verbindung zum Server ist abgerissen.');
    expect(t.close).not.toHaveBeenCalled();
    document.querySelector<HTMLButtonElement>('[data-term-again]')!.click();
    await nextTick();
    expect(FakeSocket.all).toHaveLength(2);
    expect(document.body.textContent).not.toContain('abgerissen');
    t.app.unmount();
  });

  it('ends the server side and lets go of xterm when the window closes', () => {
    const t = mount();
    t.ws().open();
    t.app.unmount();
    expect(t.ws().closedWith).toBe(1000);
    expect(t.screen().disposed).toBe(true);
  });
});

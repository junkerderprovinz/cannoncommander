import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { addMessages, setLanguage } from '@/i18n';
import de from './de.json';
import en from './en.json';
import { FakeSocket } from '@/testing/fakeSocket';
import { type Ended, link } from './link';

addMessages('terminal', { de, en });

beforeEach(() => {
  FakeSocket.all = [];
  vi.stubGlobal('WebSocket', FakeSocket);
  setLanguage('de');
});
afterEach(() => { vi.unstubAllGlobals(); });

const quiet = () => ({ open: vi.fn(), data: vi.fn(), end: vi.fn<(e: Ended) => void>() });

describe('link', () => {
  it('asks the server for a shell in the folder at the size of the terminal', () => {
    link('/mnt/user/app data', 120, 40, quiet());
    const url = new URL(FakeSocket.all[0]!.url);
    expect(url.protocol).toBe('ws:');
    expect(url.pathname).toBe('/api/v1/terminal');
    expect(Object.fromEntries(url.searchParams)).toEqual({ path: '/mnt/user/app data', cols: '120', rows: '40' });
  });

  it('sends typing as bytes, a long paste in pieces, and a new size as text', () => {
    const l = link('/mnt/user', 80, 24, quiet());
    const ws = FakeSocket.all[0]!;
    ws.open();
    l.send('ls\r');
    l.send('x'.repeat(40000));
    l.resize(100, 30);
    expect(new TextDecoder().decode(ws.sent[0] as Uint8Array)).toBe('ls\r');
    expect(ws.sent.slice(1, 4).map((s) => (s as Uint8Array).length)).toEqual([16384, 16384, 7232]);
    expect(JSON.parse(ws.sent[4] as string)).toEqual({ cols: 100, rows: 30 });
  });

  it('hands on what the shell prints', () => {
    const on = quiet();
    link('/mnt/user', 80, 24, on);
    const ws = FakeSocket.all[0]!;
    ws.open();
    ws.onmessage?.({ data: new Uint8Array([104, 105]).buffer });
    expect(on.open).toHaveBeenCalled();
    expect(on.data).toHaveBeenCalledWith(new Uint8Array([104, 105]));
  });

  it('says why the server ended the terminal', () => {
    const cases: [number, string, Ended][] = [
      [1000, 'exited', { exited: true }],
      [4001, 'idle', { exited: false, text: 'Das Terminal war 30 Minuten unbenutzt und ist jetzt zu.' }],
      [4003, 'signedOut', { exited: false, text: 'Du bist abgemeldet, darum ist das Terminal zu.' }],
      [4003, 'off', { exited: false, text: 'Ein Admin hat das Terminal in den Einstellungen ausgeschaltet.' }],
      [1001, 'stopping', { exited: false, text: 'Der Server hält gerade an, darum ist das Terminal zu.' }],
      [1006, '', { exited: false, text: 'Die Verbindung zum Server ist abgerissen.' }],
    ];
    for (const [code, reason, want] of cases) {
      const on = quiet();
      link('/mnt/user', 80, 24, on);
      FakeSocket.all.at(-1)!.serverCloses(code, reason);
      expect(on.end).toHaveBeenCalledWith(want);
    }
  });

  it('closes quietly when the window goes', () => {
    const on = quiet();
    const l = link('/mnt/user', 80, 24, on);
    const ws = FakeSocket.all[0]!;
    ws.open();
    l.close();
    ws.serverCloses(1000, 'closed');
    expect(ws.closedWith).toBe(1000);
    expect(on.end).not.toHaveBeenCalled();
  });
});

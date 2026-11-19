import { createPinia, setActivePinia } from 'pinia';
import { createApp, reactive } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { api, toItem } from '@/api/client';
import { run } from '@/core/actions';
import { host } from '@/core/host';
import { winStack } from '@/core/windows';
import { endInline, inline } from '@/files/inline';
import { addMessages } from '@/i18n';
import { usePanels } from '@/stores/panels';
import de from './de.json';
import en from './en.json';
import setup, { copyName } from './index';
import NameBody from './NameBody.vue';

addMessages('names', { de, en });

describe('copyName', () => {
  it('puts the word before the ending', () => {
    expect(copyName('notizen.txt', new Set(['notizen.txt']), 'Kopie')).toBe('notizen Kopie.txt');
  });
  it('counts on when the name is taken', () => {
    expect(copyName('notizen.txt', new Set(['notizen Kopie.txt']), 'Kopie')).toBe('notizen Kopie 2.txt');
  });
  it('starts from the original when copying a copy', () => {
    expect(copyName('notizen Kopie 2.txt', new Set(['notizen Kopie.txt', 'notizen Kopie 2.txt']), 'Kopie')).toBe('notizen Kopie 3.txt');
  });
  it('leaves a folder or a hidden file without an ending', () => {
    expect(copyName('Urlaub', new Set(), 'Kopie')).toBe('Urlaub Kopie');
    expect(copyName('.env', new Set(), 'Kopie')).toBe('.env Kopie');
  });
});

describe('Umbenennen', () => {
  const file = (name: string, dir = false) => toItem({ name, dir, size: 1, mtime: '2026-11-08T10:00:00Z', mode: '644' });
  beforeEach(async () => {
    setActivePinia(createPinia());
    history.replaceState(null, '', '/#/home/jdp');
    vi.spyOn(api, 'fsList').mockImplementation(async (path) => ({ path, items: [...['a.txt', 'b.txt'].map((n) => file(n)), file('Citizen.Vigilante.2026.German.DTSHD.DL.2160p.UHD.BluRay.DV.HDR.x265-VECTOR', true)], etag: path }));
    vi.spyOn(api, 'fsTree').mockResolvedValue({ path: '/', items: [] });
    winStack.splice(0);
    endInline();
    setup();
    await new Promise((r) => setTimeout(r, 0));
  });
  afterEach(() => {
    Object.assign(host, { desktop: false, os: '' });
    document.body.innerHTML = '';
    vi.restoreAllMocks();
  });
  const row = (idx: number) => {
    document.body.innerHTML = `<section class="panel" data-panel="0"><div class="list view-details"><div class="row" data-idx="${idx}"><span class="t">x</span></div></div></section>`;
  };
  const cursorOn = (name: string) => {
    const F = usePanels();
    F.T().cursor = F.entries().findIndex((k) => k.name === name);
    return F.T().cursor;
  };

  it('renames in the row in the desktop app', () => {
    Object.assign(host, { desktop: true, os: 'windows' });
    row(cursorOn('b.txt'));
    run('rename');
    expect(inline).toMatchObject({ panel: 0, folder: '/home/jdp', name: 'b.txt' });
    expect(winStack).toHaveLength(0);
  });

  it('keeps the window in the browser', () => {
    row(cursorOn('b.txt'));
    run('rename');
    expect(inline.panel).toBe(-1);
    expect(winStack.map((w) => w.title)).toEqual(['Umbenennen']);
  });

  it('takes the window for a row that is not drawn, such as in the column view', () => {
    Object.assign(host, { desktop: true, os: 'linux' });
    const idx = cursorOn('b.txt');
    document.body.innerHTML = `<section class="panel" data-panel="0"><div class="list view-columns"><div class="row" data-idx="${idx}"><span class="t">x</span></div></div></section>`;
    run('rename');
    expect(inline.panel).toBe(-1);
    expect(winStack).toHaveLength(1);
  });

  it("selects a file's name up to its extension in the window, as the row does", () => {
    row(cursorOn('b.txt'));
    run('rename');
    expect(winStack[0]!.props).toHaveProperty('stem', true);
  });

  it("selects all of a folder's name in the window", () => {
    row(cursorOn('Citizen.Vigilante.2026.German.DTSHD.DL.2160p.UHD.BluRay.DV.HDR.x265-VECTOR'));
    run('rename');
    expect(winStack[0]!.props).not.toHaveProperty('stem', true);
  });

  it('selects only the part before the extension of a new file, and all of a new folder', () => {
    run('newFile');
    run('newFolder');
    expect(winStack.map((w) => !!w.props?.stem)).toEqual([true, false]);
  });
});

describe('the name field', () => {
  let app: ReturnType<typeof createApp> | undefined;
  afterEach(() => {
    app?.unmount();
    document.body.innerHTML = '';
  });
  const selected = async (name: string, stem?: boolean) => {
    app = createApp(NameBody, { st: reactive({ name }), stem });
    app.mount(document.body.appendChild(document.createElement('div')));
    await new Promise((r) => setTimeout(r, 0));
    const el = document.querySelector<HTMLInputElement>('#nm')!;
    return [el.selectionStart, el.selectionEnd];
  };

  it('starts with the whole name selected, as for a bookmark', async () => {
    expect(await selected('Brief an Oma.odt')).toEqual([0, 16]);
    app?.unmount();
    expect(await selected('Citizen.Vigilante.2026.German.DTSHD.DL.2160p.UHD.BluRay.DV.HDR.x265-VECTOR')).toEqual([0, 74]);
  });

  it('selects the part before the extension when asked to', async () => {
    expect(await selected('Citizen Vigilante (2026).mkv', true)).toEqual([0, 24]);
    app?.unmount();
    expect(await selected('Citizen.Vigilante.2026.German.DTSHD.DL.2160p.UHD.BluRay.DV.HDR.x265-VECTOR.mkv', true)).toEqual([0, 74]);
    app?.unmount();
    expect(await selected('Mr. Robot (2015)', true)).toEqual([0, 16]);
    app?.unmount();
    expect(await selected('.bashrc', true)).toEqual([0, 7]);
  });
});

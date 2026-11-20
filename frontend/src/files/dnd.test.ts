import { createPinia, setActivePinia } from 'pinia';
import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Item } from '@/api/client';
import { tools } from '@/api/tools';
import { action } from '@/core/actions';
import { host } from '@/core/host';
import { addMessages } from '@/i18n';
import de from '@/features/transfer/de.json';
import en from '@/features/transfer/en.json';
import { useListing } from '@/stores/listing';
import { type Transfer, usePanels } from '@/stores/panels';
import { dragOp, installDragDrop } from './dnd';

addMessages('transfer', { de, en });

const HERE = '/home/jdp';
const item = (name: string, dir = false): Item => ({ name, dir, size: 1, time: 0, mode: '0664', owner: 'jdp', group: 'jdp' });

type Hooks = { handlePlatformFileDrop: (paths: string[], x: number, y: number) => void; invoke?: (m: string) => void };
const hooks = () => (window as unknown as { _wails: Hooks })._wails;

let dropped: unknown[][] = [];
let moved: Transfer[] = [];
let invoked: string[] = [];
let dragged: unknown[] = [];

// A panel with a folder row and the list around it, the way the files view lays them out.
function panel() {
  document.body.innerHTML = `<div class="panel" data-panel="0"><div class="list"><div class="row dir" data-idx="0"></div><div class="row" data-idx="1"></div></div></div>`;
  return { list: document.querySelector('.list')!, folder: document.querySelector('.row.dir')!, file: document.querySelector('.row:not(.dir)')! };
}

beforeAll(() => {
  setActivePinia(createPinia());
  usePanels();
  action('dropIn', { label: 'drop', run: (...args: unknown[]) => { dropped.push(args); } });
  action('dragOut', { label: 'drag', run: (paths: unknown) => { dragged.push(paths); } });
  action('transfer', { label: 'transfer', run: ((job: Transfer) => { moved.push(job); }) as never });
  (window as unknown as { _wails: Partial<Hooks> })._wails = { invoke: (m: string) => invoked.push(m) };
  installDragDrop();
});

beforeEach(() => {
  const F = usePanels();
  F.T().path = HERE;
  useListing().lists.set(HERE, { path: HERE, items: [item('Bilder', true), item('notes.txt')] });
  useListing().lists.set(HERE + '/Bilder', { path: HERE + '/Bilder', items: [] });
  Object.assign(host, { desktop: true, os: 'linux' });
  dropped = [];
  moved = [];
  dragged = [];
  vi.stubGlobal('fetch', vi.fn(async () => new Response(null, { status: 304 })));
});

describe('the window of the desktop app', () => {
  it('hears that the page takes its drops', () => {
    expect(invoked).toEqual(['wails:runtime:ready']);
  });

  it('drops files from another program into the folder under the pointer', () => {
    const { folder, file } = panel();
    document.elementFromPoint = () => folder;
    hooks().handlePlatformFileDrop(['/tmp/a.txt'], 10, 10);
    document.elementFromPoint = () => file;
    hooks().handlePlatformFileDrop(['/tmp/b.txt'], 10, 30);
    expect(dropped).toEqual([[['/tmp/a.txt'], HERE + '/Bilder', 'auto'], [['/tmp/b.txt'], HERE, 'auto']]);
  });

  it('hands a drag of files on disk to the system drag on Linux', () => {
    const { file } = panel();
    const start = Object.assign(new Event('dragstart', { bubbles: true, cancelable: true }), { dataTransfer: { setData: () => {}, effectAllowed: '' } });
    file.dispatchEvent(start);
    expect(start.defaultPrevented).toBe(true);
    expect(dragged).toEqual([[HERE + '/notes.txt']]);
  });

  it('takes the drop of a drag of its own on Linux as a drag inside the panels', async () => {
    const { folder, file } = panel();
    file.dispatchEvent(Object.assign(new Event('dragstart', { bubbles: true }), { dataTransfer: { setData: () => {}, effectAllowed: '' } }));
    document.elementFromPoint = () => folder;
    hooks().handlePlatformFileDrop([HERE + '/notes.txt'], 10, 10);
    await new Promise((r) => setTimeout(r, 0));
    expect(dropped).toEqual([]);
    expect(moved.map((j) => [j.kind, j.dest, j.items.map((k) => k.name)])).toEqual([['copy', HERE + '/Bilder', ['notes.txt']]]);
  });

  it('copies from where the drag started after holding still opened the target', async () => {
    const { file } = panel();
    const go = document.createElement('div');
    go.dataset.go = HERE + '/Bilder';
    document.body.append(go);
    file.dispatchEvent(Object.assign(new Event('dragstart', { bubbles: true }), { dataTransfer: { setData: () => {}, effectAllowed: '' } }));
    usePanels().T().path = HERE + '/Bilder';
    go.dispatchEvent(Object.assign(new Event('drop', { bubbles: true, cancelable: true }), { dataTransfer: { files: [], types: [] } }));
    await new Promise((r) => setTimeout(r, 0));
    expect(moved.map((j) => [j.from, j.dest, j.items.map((k) => k.name)])).toEqual([[HERE, HERE + '/Bilder', ['notes.txt']]]);
  });

  it('drops onto a drive in This PC like onto any folder', () => {
    usePanels().T().path = '/';
    useListing().lists.set('/', { path: '/', items: [item('C', true), item('net', true)] });
    document.body.innerHTML = `<div class="panel" data-panel="0"><div class="list"><div class="row dir dcard" data-idx="0"></div><div class="row dir dcard" data-idx="1"></div></div></div>`;
    document.elementFromPoint = () => document.querySelector('.dcard');
    hooks().handlePlatformFileDrop(['C:\\Temp\\a.txt'], 10, 10);
    expect(dropped).toEqual([[['C:\\Temp\\a.txt'], '/C', 'auto']]);
  });
});

const keys = (k: Partial<Record<'ctrlKey' | 'shiftKey' | 'altKey' | 'metaKey', boolean>> = {}) => ({ ctrlKey: false, shiftKey: false, altKey: false, metaKey: false, ...k });

describe('the keys held during a drag', () => {
  it('copy without a key, copy with Ctrl, move with Shift and link with Alt or Ctrl and Shift', () => {
    expect([keys(), keys({ ctrlKey: true }), keys({ shiftKey: true }), keys({ altKey: true }), keys({ ctrlKey: true, shiftKey: true })].map((k) => dragOp(k, false, true)))
      .toEqual(['copy', 'copy', 'move', 'link', 'link']);
  });

  it('copy with Option, move with Command and link with both on a Mac', () => {
    expect([keys({ altKey: true }), keys({ metaKey: true }), keys({ altKey: true, metaKey: true }), keys({ shiftKey: true })].map((k) => dragOp(k, true, true)))
      .toEqual(['copy', 'move', 'link', 'move']);
  });

  it('copy instead of linking where a link cannot go', () => {
    expect(dragOp(keys({ altKey: true }), false, false)).toBe('copy');
  });
});

describe('a drag inside the browser', () => {
  const ev = (type: string, k: Partial<Record<'ctrlKey' | 'shiftKey' | 'altKey' | 'metaKey', boolean>> = {}) =>
    Object.assign(new Event(type, { bubbles: true, cancelable: true }), { dataTransfer: { setData: () => {}, effectAllowed: '', dropEffect: '', types: [], files: [] }, clientX: 40, clientY: 40, ...keys(k) });
  const badge = () => document.querySelector('.drag-op')?.textContent?.trim();
  beforeEach(() => Object.assign(host, { desktop: false, os: 'linux' }));

  it('shows what a drop would do and follows the keys while the drag goes on', () => {
    const { folder, file } = panel();
    file.dispatchEvent(ev('dragstart'));
    const plain = ev('dragover');
    folder.dispatchEvent(plain);
    expect([plain.dataTransfer.dropEffect, badge()]).toEqual(['copy', 'Kopieren']);
    const shifted = ev('dragover', { shiftKey: true });
    folder.dispatchEvent(shifted);
    expect([shifted.dataTransfer.dropEffect, badge()]).toEqual(['move', 'Verschieben']);
    const ctrl = ev('dragover', { ctrlKey: true });
    folder.dispatchEvent(ctrl);
    expect([ctrl.dataTransfer.dropEffect, badge()]).toEqual(['copy', 'Kopieren']);
    file.dispatchEvent(ev('dragend'));
    expect(badge()).toBeUndefined();
  });

  it('does what the keys say at the moment of the drop', async () => {
    const { folder, file } = panel();
    file.dispatchEvent(ev('dragstart'));
    folder.dispatchEvent(ev('dragover'));
    folder.dispatchEvent(ev('drop', { shiftKey: true }));
    await new Promise((r) => setTimeout(r, 0));
    expect(moved.map((j) => [j.kind, j.dest])).toEqual([['move', HERE + '/Bilder']]);
    expect(badge()).toBeUndefined();
  });

  it('links with Alt', async () => {
    const linked = vi.spyOn(tools, 'symlink').mockResolvedValue({ path: HERE + '/Bilder/notes.txt' });
    const { folder, file } = panel();
    file.dispatchEvent(ev('dragstart'));
    const over = ev('dragover', { altKey: true });
    folder.dispatchEvent(over);
    expect([over.dataTransfer.dropEffect, badge()]).toEqual(['link', 'Verknüpfen']);
    folder.dispatchEvent(ev('drop', { altKey: true }));
    await new Promise((r) => setTimeout(r, 0));
    expect(linked).toHaveBeenCalledWith(HERE + '/notes.txt', HERE + '/Bilder/notes.txt');
    expect(moved).toEqual([]);
  });
});

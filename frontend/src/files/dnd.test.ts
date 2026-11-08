import { createPinia, setActivePinia } from 'pinia';
import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Item } from '@/api/client';
import { action } from '@/core/actions';
import { host } from '@/core/host';
import { useListing } from '@/stores/listing';
import { type Transfer, usePanels } from '@/stores/panels';
import { installDragDrop } from './dnd';

const HERE = '/home/jdp';
const item = (name: string, dir = false): Item => ({ name, dir, size: 1, time: 0, mode: '0664', owner: 'jdp', group: 'jdp' });

type Hooks = { handlePlatformFileDrop: (paths: string[], x: number, y: number) => void; invoke?: (m: string) => void };
const hooks = () => (window as unknown as { _wails: Hooks })._wails;

let dropped: unknown[][] = [];
let moved: Transfer[] = [];
let invoked: string[] = [];

// A panel with a folder row and the list around it, the way the files view lays them out.
function panel() {
  document.body.innerHTML = `<div class="panel" data-panel="0"><div class="list"><div class="row dir" data-idx="0"></div><div class="row" data-idx="1"></div></div></div>`;
  return { list: document.querySelector('.list')!, folder: document.querySelector('.row.dir')!, file: document.querySelector('.row:not(.dir)')! };
}

beforeAll(() => {
  setActivePinia(createPinia());
  usePanels();
  action('dropIn', { label: 'drop', run: (...args: unknown[]) => { dropped.push(args); } });
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

  it('takes the drop of a drag of its own on Linux as a drag inside the panels', async () => {
    const { folder, file } = panel();
    file.dispatchEvent(Object.assign(new Event('dragstart', { bubbles: true }), { dataTransfer: { setData: () => {}, effectAllowed: '' } }));
    file.dispatchEvent(Object.assign(new Event('dragend', { bubbles: true })));
    document.elementFromPoint = () => folder;
    hooks().handlePlatformFileDrop([HERE + '/notes.txt'], 10, 10);
    await new Promise((r) => setTimeout(r, 0));
    expect(dropped).toEqual([]);
    expect(moved.map((j) => [j.kind, j.dest, j.items.map((k) => k.name)])).toEqual([['copy', HERE + '/Bilder', ['notes.txt']]]);
  });
});

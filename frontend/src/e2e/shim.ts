// The prototype's globals in the test build. The step files in prototype/tools reach into the page through CC,
// Files, FS, Jobs, DATA and a few helpers; this file puts the same names on window and answers them from the
// core services and the stores, so one step file drives the prototype and the app alike.
// main.ts loads it only when VITE_E2E is 1. Parts that later packages build announce themselves with provide().
import {
  actions, applyPrefs, blocked, closeMenu, closeWin, fmtSize, go as goView, keymap, MENUS, prefs, run, setPref, view,
  winStack,
} from '@/core';
import { DAY } from '@/core/format';
import { provideOps } from './ops';
import { provideTools } from './tools';
import { menuName } from '@/core/menus';
import { orderedTabs } from '@/core/views';
import { COLUMNS, Columns, DEFAULT_COLUMNS } from '@/files/columns';
import { join, parent, split } from '@/files/paths';
import type { Item } from '@/api/client';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';
import { useTrash } from '@/stores/trash';
import { useUser } from '@/stores/user';

// The moment the prototype's made-up data and the fixture on disk were made; the parity runner sets the page clock to it.
const NOW = Date.UTC(2026, 10, 2, 9, 30);

interface FsNode extends Item {
  children?: FsNode[];
}

type Bag = Record<string, unknown>;
// Members handed in before the shim runs wait here; afterwards they go straight onto the live objects.
const waiting: Record<string, Bag> = {};
const live: Record<string, Bag> = {};

/**
 * Hands the shim a global or a member of one, for the parts the prototype has and the app builds later:
 * provide('Jobs', jobs), provide('CC.unraid', unraid), provide('DATA.shares', shares).
 */
export function provide(name: string, value: unknown) {
  const [owner, member] = name.split('.') as [string, string?];
  if (member) (live[owner] ?? (waiting[owner] ??= {}))[member] = value;
  else Object.defineProperty(window, owner, { value, configurable: true, writable: true });
}

// The prototype's FS over the listing cache, built on demand, so a folder the app has read is a folder FS knows.
function fsOver(listing: ReturnType<typeof useListing>) {
  const nodeOf = (item: Item, path: string): FsNode => (item.dir
    ? { ...item, get children() { return (listing.lists.get(path)?.items || []).map((k) => nodeOf(k, join(path, k.name))); } }
    : { ...item });
  const root = nodeOf({ name: '', dir: true, size: 0, time: 0, mode: '0755', owner: 'root', group: 'root' }, '/');
  const FS = {
    root,
    split,
    join,
    parent,
    nodeAt(path: string): FsNode | null {
      let node: FsNode | undefined = root;
      for (const part of split(path)) {
        node = node.children?.find((k) => k.name === part);
        if (!node) return null;
      }
      return node;
    },
    kids: (n: FsNode | null) => n?.children || [],
    totalSize: (n: FsNode): number => (n.dir ? FS.kids(n).reduce((a, k) => a + FS.totalSize(k), 0) : n.size || 0),
    inArchive: (path: string) => path.includes('::'),
  };
  return FS;
}

// The step files read folders synchronously, as the prototype keeps its whole tree in memory, so every folder below
// the root is read before the first step runs.
async function readAll(listing: ReturnType<typeof useListing>, path = '/') {
  const got = await listing.load(path).catch(() => null);
  for (const k of got?.items || []) if (k.dir && !k.link) await readAll(listing, join(path, k.name));
}

export async function installShim() {
  const listing = useListing(), panels = usePanels(), user = useUser();
  provideOps(provide);
  provideTools(provide);
  await readAll(listing);

  const CC = {
    get actions() { return actions; },
    run,
    blocked,
    keymap,
    prefs,
    setPref,
    applyPrefs,
    // Vue draws on its own after every change; the step files still ask for it after setting state by hand.
    render: () => {},
    closeWin,
    go: goView,
    get view() { return view.id; },
    orderedTabs,
    get signedIn() { return !!user.me.name; },
    ...waiting.CC,
  };

  const Files = {
    get active() { return panels.active; },
    set active(i: number) { panels.active = i; },
    T: panels.T,
    P: panels.P,
    entries: panels.entries,
    sel: panels.sel,
    cursorEntry: panels.cursorEntry,
    get openTree() { return panels.openTree; },
    // The prototype changes folder at once; the store waits for the server. A folder already read changes at once
    // here too, so a step that sets the cursor right after going there sets it in the new folder.
    go(path: string, i = panels.active) {
      if (listing.lists.has(path)) panels.enter(path, i);
      else void panels.go(path, i);
    },
    ...waiting.Files,
  };

  const DATA = {
    get bookmarks() { return user.bookmarks; },
    get trash() { return useTrash().items; },
    ...waiting.DATA,
  };

  Object.assign(live, { CC, Files, DATA });
  const globals: Record<string, unknown> = {
    CC,
    Files,
    DATA,
    FS: fsOver(listing),
    Columns,
    COLUMNS,
    viewOf: panels.viewOf,
    DEFAULT_COLUMNS,
    // The step files find a menu by its German title, as the prototype names them; the id stands third.
    MENUS: MENUS.map(([id, items], i) => [menuName(i), items, id]),
    winStack,
    closeMenu,
    $: (s: string, r: ParentNode = document) => r.querySelector(s),
    $$: (s: string, r: ParentNode = document) => [...r.querySelectorAll(s)],
    fmtSize,
    NOW,
    DAY,
  };
  for (const [name, value] of Object.entries(globals)) Object.defineProperty(window, name, { value, configurable: true, writable: true });
  document.documentElement.dataset.e2e = 'ready';
}

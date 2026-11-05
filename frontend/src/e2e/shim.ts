// The prototype's globals in the test build. The step files in prototype/tools reach into the page through CC,
// Files, FS, Jobs, DATA and a few helpers; this file puts the same names on window and answers them from the
// core services and the stores, so one step file drives the prototype and the app alike.
// main.ts loads it only when VITE_E2E is 1. Parts that later packages build announce themselves with provide().
import {
  actions, applyPrefs, blocked, closeMenu, closeWin, fmtSize, go as goView, keymap, MENUS, prefs, run, setPref, view,
  winStack,
} from '@/core';
import { DAY } from '@/core/format';
import { installHooks } from './hooks';
import { provideOps } from './ops';
import { devOf, provideUnraid } from './unraid';
import { provideTools } from './tools';
import { menuName } from '@/core/menus';
import { orderedTabs, settingsTabs } from '@/core/views';
import { hashPath } from '@/core/nav';
import { type Action, action, labelOf } from '@/core/actions';
import { glyph, kindOf } from '@/core/glyphs';
import { BASE, toItem } from '@/api/client';
import type { FsList } from '@/api/types';
import type { Activity, Session, Source } from '@/api/admin';
import type { Share } from '@/api/tools';
import { watch } from 'vue';
import { device } from '@/features/config/sessions';
import { session } from '@/signin';
import { auth } from '@/api/auth';
import { topFolders } from '@/files/frequent';
import { type Clip, useClipboard } from '@/stores/clipboard';
import { byName, type Colour, type Label, type Tag, useLabels } from '@/stores/labels';
import type { RowMark } from '@/stores/panels';
import { COLUMNS, Columns, DEFAULT_COLUMNS } from '@/files/columns';
import { join, parent, split } from '@/files/paths';
import type { Item } from '@/api/client';
import { useListing } from '@/stores/listing';
import { newTab, usePanels } from '@/stores/panels';
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
// A node also carries the disk it lies on and a file its text, as the prototype's do, each read only when a step
// asks for it.
const pathOf = new WeakMap<object, string>();
// A folder made after the start, which no panel has shown yet, is asked for at once.
const listNow = (path: string) => (getNow<FsList>('/fs/list?path=' + encodeURIComponent(path))?.items ?? []).map(toItem);

function fsOver(listing: ReturnType<typeof useListing>) {
  const nodeOf = (item: Item, path: string): FsNode => {
    const node: FsNode = item.dir
      ? { ...item, get children() { return (listing.lists.get(path)?.items ?? listNow(path)).map((k) => nodeOf(k, join(path, k.name))); } }
      : { ...item };
    pathOf.set(node, path);
    if (!item.dir) Object.defineProperty(node, 'text', { get: () => getNow<{ text: string }>(`/fs/text?path=${encodeURIComponent(path)}&max=1048576`)?.text });
    return Object.defineProperty(node, 'dev', { get: () => devOf(path) });
  };
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

// The step files read some lists at once from the prototype's memory; here they come from the server the same way.
function getNow<T>(path: string): T | undefined {
  const req = new XMLHttpRequest();
  req.open('GET', BASE + path, false);
  req.send();
  return req.status === 200 ? (JSON.parse(req.responseText) as T) : undefined;
}

// GlimStone names some parts of its menus otherwise than the prototype did, and the step files still look for the old
// names: the pencil in a heading, a row that opens a submenu and the chosen one of a set. The Google Drive tile of the
// remote window carries rclone's name for it, drive, where the prototype said gdrive. A damaged archive carries its
// mark on the name, where the prototype marked the whole row.
function aliasMenuParts() {
  const alias = () => {
    document.querySelectorAll<HTMLElement>('.glim-menu-head-edit').forEach((el) => {
      el.classList.add('hedit');
      el.dataset.tip ||= el.getAttribute('aria-label') || el.getAttribute('title') || '';
    });
    document.querySelectorAll<HTMLElement>('.glim-menu [aria-haspopup="menu"]').forEach((el) => { el.dataset.sub ??= ''; });
    document.querySelectorAll('.row').forEach((el) => el.classList.toggle('damaged', !!el.querySelector('.bk-damaged')));
    document.querySelectorAll<HTMLElement>('.a-tile[data-type="drive"]').forEach((el) => { el.dataset.type = 'gdrive'; });
    document.querySelectorAll('.glim-menu [role="menuitemradio"]').forEach((el) => el.classList.toggle('on', el.getAttribute('aria-checked') === 'true'));
  };
  new MutationObserver(alias).observe(document.body, { childList: true, subtree: true });
}

export async function installShim() {
  installHooks();
  aliasMenuParts();
  const listing = useListing(), panels = usePanels(), user = useUser(), clipboard = useClipboard(), labels = useLabels();
  provideOps(provide);
  provideUnraid(provide, (n) => pathOf.get(n));
  provideTools(provide);
  await readAll(listing);
  // Signing out reloads into the sign in, which would take the shim with it. Here the app stays behind the sign in, as
  // the prototype keeps it, and signing in again brings it back as it was.
  const logout = actions.logout;
  if (logout) action('logout', { ...logout, run: async () => { await auth.logout().catch(() => {}); session.out = true; } });
  // The prototype starts with two tabs on the left and the backups on the right, and the step files count on it. A
  // folder in the address, as after a reload, takes the first tab's place.
  [[hashPath() || '/mnt/user/downloads', '/mnt/user/fotos/2026'], ['/mnt/user/backups']].forEach((paths, i) => Object.assign(panels.P(i), { tabs: paths.map(newTab), cur: 0 }));
  panels.openTree.clear();
  for (const p of ['/mnt', '/mnt/user']) panels.openTree.add(p);

  // The prototype's actions carry their label as text, but for the collection, whose label counts what it holds; the
  // app's work it out in the language of the moment.
  const asFunction = new Set(['collection']);
  // The plumbing the app runs as actions with what a caller hands it stays out of the list the step files walk.
  const plumbing = new Set(['transfer', 'uploadFiles']);
  const shownActions = new Proxy(actions, {
    get: (all, id) => {
      const a = Reflect.get(all, id) as Action | undefined;
      return a && typeof a === 'object' ? { ...a, label: asFunction.has(a.id) ? () => labelOf(a) : labelOf(a) } : a;
    },
    ownKeys: (all) => Reflect.ownKeys(all).filter((id) => !plumbing.has(id as string)),
  });
  const reading = () => panels.panels.some((_, i) => !panels.loaded(i));
  const panelsRead = () => new Promise<void>((done) => {
    const stop = watch(reading, (still) => { if (!still) { stop(); done(); } });
    setTimeout(() => { stop(); done(); }, 5000);
  });
  const CC = {
    get actions() { return shownActions; },
    // The prototype filters and searches at once; the app asks its server, so an action waits for what the panels show.
    run: (id: string, ...args: unknown[]) => (reading() ? panelsRead().then(() => run(id, ...args)) : run(id, ...args)),
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
    orderedTabs: () => orderedTabs(settingsTabs),
    get settingsTab_() { return view.id === 'settings' ? document.querySelector<HTMLElement>('[data-stab][aria-selected="true"]')?.dataset.stab : undefined; },
    topFolders: (n?: number, skip?: string) => topFolders(n, skip),
    admin: { ddVal: (el: HTMLElement | null) => el?.dataset.val },
    live: { demo: (run: boolean) => fetch(BASE + '/fixture/download', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ run }) }) },
    get signedIn() { return !!user.me.name && !session.out; },
    // Signing out in a step shows the sign in over the app, as when a session runs out.
    set signedIn(on: boolean) {
      const req = new XMLHttpRequest();
      req.open('PUT', BASE + '/fixture/session', false);
      req.setRequestHeader('Content-Type', 'application/json');
      req.send(JSON.stringify({ signedIn: on }));
      session.out = !on;
    },
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
    get clip() { return clipboard.clip; },
    set clip(c: Clip | null) { clipboard.clip = c; },
    get marks() { return panels.marks; },
    set marks(m: Record<string, RowMark>[]) { panels.marks.splice(0, panels.marks.length, ...m); },
    // The prototype changes folder at once; the store waits for the server. Here the folder is read at once when it
    // is not in the cache, so a step that sets the cursor right after going there sets it in the new folder.
    go(path: string, i = panels.active) {
      if (!listing.lists.has(path)) {
        const got = getNow<FsList>('/fs/list?path=' + encodeURIComponent(path));
        if (!got) return void panels.go(path, i);
        listing.lists.set(path, { path, items: got.items.map(toItem), volume: got.volume });
      }
      panels.enter(path, i);
    },
    ...waiting.Files,
  };

  const DATA = {
    get bookmarks() { return user.bookmarks; },
    get trash() { return useTrash().items; },
    labels: new Proxy({}, {
      get: (_, path: string) => {
        const l = getNow<{ items: Record<string, Label> }>('/labels/items?path=' + encodeURIComponent(parent(path)))?.items[split(path).pop()!];
        return l && { color: l.colour, tags: l.tags || [] };
      },
    }),
    get sources() { return getNow<{ sources: Source[] }>('/admin/sources')?.sources ?? []; },
    get sessions() { return (getNow<{ items: Session[] }>('/me/sessions')?.items ?? []).map((s) => ({ ...s, device: device(s.userAgent).name })); },
    get activity() { return getNow<{ items: Activity[] }>('/admin/activity?limit=500')?.items ?? []; },
    get shares() { return (getNow<Share[]>('/shares') ?? []).map((s) => ({ ...s, path: s.paths[0] })); },
    ...waiting.DATA,
  };

  Object.assign(live, { CC, Files, DATA });
  const globals: Record<string, unknown> = {
    CC,
    Files,
    DATA,
    FS: { ...fsOver(listing), kindOf },
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
    esc: (s: unknown) => String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c]!),
    g: (n: string) => { const d = glyph(n); return `<svg viewBox="${d.viewBox}" fill="currentColor" aria-hidden="true">${d.svg}</svg>`; },
    NOW,
    DAY,
  };
  for (const [name, value] of Object.entries(globals)) Object.defineProperty(window, name, { value, configurable: true, writable: true });
  // The colours and tags as the server keeps them right now, in the prototype's shapes.
  const labelsNow = () => getNow<{ colours: Colour[]; tags: Tag[] }>('/labels') ?? { colours: [], tags: [] };
  Object.defineProperty(window, 'LABEL_COLORS', { configurable: true, get: () => labelsNow().colours.map((c) => [c.id, labels.colourName(c), c.hex]) });
  Object.defineProperty(window, 'Labels', { configurable: true, value: { tags: () => labelsNow().tags.map((x) => x.name).sort(byName) } });
  document.documentElement.dataset.e2e = 'ready';
}

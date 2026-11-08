// Dragging rows copies onto a folder, a tab, a piece of the path, the sidebar or the other panel; Shift moves.
// Holding still over a folder, tab or path piece opens it, so a drop can go deeper, as in Finder.
// Files dragged in from outside go to the upload. In the desktop app they are files on the same computer instead: they
// go onto the folder under the pointer as Explorer takes them, and rows dragged out of the window reach Explorer, the
// desktop or any other program as the files themselves.
import { run } from '@/core/actions';
import { host } from '@/core/host';
import { prefs } from '@/core/prefs';
import { view } from '@/core/views';
import { usePanels } from '@/stores/panels';
import type { Entry } from './entries';
import { inArchive, join, nativePath } from './paths';

interface Target { el: Element; dest: string; open?: () => void }
interface Drag { from: number; items: Entry[]; folder: string; shift: boolean; native?: boolean }

// The window of the desktop app hands over dropped files through the hooks of Wails' runtime. The page defines them
// itself, since the runtime would come from the page's own server, and says it is ready, after which the window
// calls them.
interface WailsHooks {
  invoke?: (msg: string) => void;
  handlePlatformFileDrop?: (paths: string[], x: number, y: number) => void;
  handleDragEnter?: () => void;
  handleDragOver?: (x: number, y: number) => void;
  handleDragLeave?: () => void;
}
type WebView = { postMessageWithAdditionalObjects?: (msg: string, objs: unknown[]) => void };
const shellOf = () => window as unknown as { _wails?: WailsHooks; chrome?: { webview?: WebView } };

export function installDragDrop() {
  const F = usePanels();
  let drag: Drag | null = null;
  // A drag that just ended, for the drop of its own files that the window on Linux reports a moment later.
  let ended: { drag: Drag; at: number } | null = null;
  // Drops on Windows wait here for their paths, which come back from the window after the page passed it the files.
  const waiting: { dest: string; how: 'copy' | 'move' | 'auto' }[] = [];
  let spring: { el: Element | null; timer: ReturnType<typeof setTimeout> | undefined } = { el: null, timer: undefined };

  const panelOf = (el: Element | null) => (el ? +(el as HTMLElement).dataset.panel! : F.active);
  // Inside the app a drag skips its own folder and its own rows; one from outside lands on any folder.
  function dropTarget(el: Element | null, d: Drag | null): Target | null {
    const at = (sel: string) => el?.closest?.(sel) ?? null;
    const panelEl = at('.panel[data-panel]'), panel = panelOf(panelEl);
    const row = at('.row.dir') as HTMLElement | null;
    if (row && panelEl) {
      const k = F.entries(panel)[+row.dataset.idx!];
      if (k && !d?.items.some((x) => x.name === k.name && (x.at || '') === (k.at || ''))) {
        const dest = join(k.at || F.T(panel).path, k.name);
        return { el: row, dest, open: () => F.go(dest, panel) };
      }
    }
    const tab = at('[data-tab]') as HTMLElement | null;
    if (tab && panelEl) {
      const n = +tab.dataset.tab!;
      return { el: tab, dest: F.P(panel).tabs[n]!.path, open: () => { F.P(panel).cur = n; } };
    }
    const go = at('[data-go]') as HTMLElement | null;
    if (go) return { el: go, dest: go.dataset.go!, open: () => F.go(go.dataset.go!, panelEl ? panel : F.active) };
    const list = at('.list');
    if (list && (!d || panel !== d.from)) return { el: list, dest: F.T(panel).path };
    return null;
  }
  const clearDrop = () => document.querySelectorAll('.dragover, .drop').forEach((x) => x.classList.remove('dragover', 'drop'));
  const stopSpring = () => {
    clearTimeout(spring.timer);
    spring = { el: null, timer: undefined };
  };
  function hover(tgt: Target | null) {
    if (!tgt) return clearDrop();
    if (!tgt.el.classList.contains('drop') && !tgt.el.classList.contains('dragover')) {
      clearDrop();
      tgt.el.classList.add(tgt.el.classList.contains('list') ? 'dragover' : 'drop');
    }
    if (spring.el !== tgt.el) {
      clearTimeout(spring.timer);
      spring = { el: tgt.el, timer: tgt.open ? setTimeout(() => { tgt.open!(); spring.el = null; }, 900) : undefined };
    }
  }
  function endDrag() {
    if (drag && !drag.native) ended = { drag, at: Date.now() };
    drag = null;
    stopSpring();
    clearDrop();
  }
  const paths = (d: Drag) => d.items.map((k) => join(k.at || d.folder, k.name));

  document.addEventListener('dragstart', (e) => {
    const row = (e.target as Element).closest?.<HTMLElement>('.row');
    const panelEl = row?.closest<HTMLElement>('.panel[data-panel]');
    if (!row || !panelEl || view.id !== 'files') return;
    F.active = +panelEl.dataset.panel!;
    const tab = F.T(), k = F.entries()[+row.dataset.idx!];
    if (!k) return;
    if (!tab.marked.has(k.name)) tab.marked = new Set([k.name]);
    drag = { from: F.active, items: F.sel(), folder: tab.path, shift: e.shiftKey };
    const onDisk = host.desktop && !inArchive(tab.path) && drag.items.every((k) => !inArchive(k.at || ''));
    // WebView2 drags only within the page, so on Windows the shell's own drag carries the files, inside the window as
    // well, where the page sees it like any other and drops it the usual way.
    if (onDisk && host.os === 'windows') {
      e.preventDefault();
      const d = drag;
      d.native = true;
      void Promise.resolve(run('dragOut', paths(d))).finally(() => { if (drag === d) endDrag(); });
      return;
    }
    e.dataTransfer!.effectAllowed = 'copyMove';
    e.dataTransfer!.setData('text/plain', drag.items.map((x) => x.name).join('\n'));
    // WebKitGTK hands a URI list on to GTK, where Nautilus, Dolphin and the desktop take it as files.
    if (onDisk) e.dataTransfer!.setData('text/uri-list', paths(drag).map((p) => 'file://' + encodeURI(nativePath(p, false)).replace(/[?#]/g, encodeURIComponent)).join('\r\n'));
  });
  document.addEventListener('dragover', (e) => {
    if (drag) drag.shift = e.shiftKey;
    const files = [...(e.dataTransfer?.types || [])].includes('Files');
    if (!drag && files && !host.desktop) {
      const list = (e.target as Element).closest?.('.list');
      if (!list) return;
      e.preventDefault();
      clearDrop();
      list.classList.add('dragover');
      return;
    }
    const tgt = dropTarget(e.target as Element, drag);
    hover(drag || files ? tgt : null);
    if (!tgt || (!drag && !files)) return;
    e.preventDefault();
    // Files from another program are always taken as a copy there, so it never deletes them after a move: the app
    // moves them itself where it should.
    e.dataTransfer!.dropEffect = !drag ? 'copy' : e.shiftKey ? 'move' : 'copy';
  });
  document.addEventListener('dragend', endDrag);
  document.addEventListener('drop', (e) => {
    clearDrop();
    stopSpring();
    const files = e.dataTransfer?.files, shell = shellOf().chrome?.webview;
    if (host.desktop && !drag && files?.length && shell?.postMessageWithAdditionalObjects) {
      e.preventDefault();
      const tgt = dropTarget(e.target as Element, null);
      if (!tgt) return;
      waiting.push({ dest: tgt.dest, how: e.shiftKey ? 'move' : e.ctrlKey ? 'copy' : 'auto' });
      shell.postMessageWithAdditionalObjects(`file:drop:${Math.round(e.clientX)}:${Math.round(e.clientY)}`, [...files]);
      return;
    }
    const list = (e.target as Element).closest?.('.list');
    if (!drag && list && files?.length) {
      e.preventDefault();
      F.active = panelOf(list.closest('.panel[data-panel]'));
      run('uploadFiles', [...files], F.T().path);
      return;
    }
    if (!drag) return;
    e.preventDefault();
    const d = drag, tgt = dropTarget(e.target as Element, d);
    if (!d.native) drag = null;
    if (tgt) F.transfer(e.shiftKey ? 'move' : 'copy', d.items, tgt.dest, d.folder);
  });

  // The window's side of a drop. On Windows the page passed it the files and gets back their paths. On Linux the
  // window takes every drop with files itself, the app's own drags too, and the page only learns where it landed.
  function dropped(list: string[], x: number, y: number) {
    clearDrop();
    stopSpring();
    const next = waiting.shift();
    if (next) {
      run('dropIn', list, next.dest, next.how);
      return;
    }
    const own = drag ?? (ended && Date.now() - ended.at < 3000 ? ended.drag : null);
    const el = document.elementFromPoint(x, y);
    if (own && list.length === own.items.length) {
      endDrag();
      ended = null;
      const tgt = dropTarget(el, own);
      if (tgt) F.transfer(own.shift ? 'move' : 'copy', own.items, tgt.dest, own.folder);
      return;
    }
    const tgt = dropTarget(el, null);
    if (tgt) run('dropIn', list, tgt.dest, 'auto');
  }
  const wails = (shellOf()._wails ??= {});
  Object.assign(wails, {
    handlePlatformFileDrop: dropped,
    handleDragEnter: () => {},
    handleDragOver: (x: number, y: number) => { if (!drag) hover(dropTarget(document.elementFromPoint(x, y), null)); },
    handleDragLeave: () => { if (!drag) { stopSpring(); clearDrop(); } },
  } satisfies WailsHooks);
  const ready = () => wails.invoke?.('wails:runtime:ready');
  if (wails.invoke) ready();
  else window.addEventListener('wails:runtime-config-ready', ready, { once: true });

  // On a phone only one panel fits; a sideways swipe over the list switches to the other one.
  let swipe: { x: number; y: number } | null = null;
  document.addEventListener('touchstart', (e) => {
    swipe = view.id === 'files' && prefs.dual && (e.target as Element).closest?.('.panel') && e.touches[0] ? { x: e.touches[0].clientX, y: e.touches[0].clientY } : null;
  }, { passive: true });
  document.addEventListener('touchend', (e) => {
    const end = e.changedTouches[0];
    if (!swipe || !end || !matchMedia('(max-width: 1100px)').matches) return;
    const dx = end.clientX - swipe.x, dy = end.clientY - swipe.y;
    swipe = null;
    if (Math.abs(dx) < 60 || Math.abs(dy) > Math.abs(dx) / 2) return;
    F.active = 1 - F.active;
  });
}

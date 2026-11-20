// Dragging rows onto a folder, a tab, a piece of the path, the sidebar or the other panel copies them. Shift moves and
// Alt links, and a badge at the pointer says which, following the keys while the drag goes on. A folder under the drag
// only lights up as the target and never opens. Files dragged in from outside go to the upload. In the
// desktop app they are files on the same computer instead: they go onto the folder under the pointer, and rows dragged
// out of the window reach the desktop or any other program as the files themselves.
import { tools } from '@/api/tools';
import { run } from '@/core/actions';
import { glyph } from '@/core/glyphs';
import { host } from '@/core/host';
import { toast } from '@/core/toast';
import { prefs } from '@/core/prefs';
import { view } from '@/core/views';
import { plural, t } from '@/i18n';
import { usePanels } from '@/stores/panels';
import type { Entry } from './entries';
import { inArchive, join, nativePath } from './paths';

interface Target { el: Element; dest: string }
interface Drag { from: number; items: Entry[]; folder: string; op: DragOp; native?: boolean }

export type DragOp = 'copy' | 'move' | 'link';
type Keys = Pick<KeyboardEvent, 'ctrlKey' | 'shiftKey' | 'altKey' | 'metaKey'>;

// Ctrl copies, Shift moves, Alt or Ctrl with Shift links. A Mac has its own keys: Option copies, Command moves and both
// together link. Without a key, or where a link cannot go, a drag copies.
export function dragOp(k: Keys, mac: boolean, linkable: boolean): DragOp {
  if (linkable && (mac ? k.altKey && k.metaKey : k.altKey || (k.ctrlKey && k.shiftKey))) return 'link';
  if (mac ? k.altKey : k.ctrlKey) return 'copy';
  return k.shiftKey || (mac && k.metaKey) ? 'move' : 'copy';
}
const onMac = () => host.os === 'darwin' || /^Mac/.test(navigator.platform);
const OPS: Record<DragOp, [label: string, glyph: string]> = { copy: ['transfer.copy', 'IconCopy'], move: ['transfer.move', 'IconMove'], link: ['transfer.symlink', 'IconLink'] };

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
  let badge: HTMLElement | null = null;
  function showOp(op: DragOp, x: number, y: number) {
    if (!badge) badge = document.body.appendChild(Object.assign(document.createElement('div'), { className: 'drag-op glim-card' }));
    if (badge.dataset.op !== op) {
      const [label, g] = OPS[op], d = glyph(g);
      badge.dataset.op = op;
      badge.innerHTML = `<svg viewBox="${d.viewBox}" fill="currentColor" aria-hidden="true">${d.svg}</svg><span></span>`;
      badge.querySelector('span')!.textContent = t(label);
    }
    badge.style.translate = `${x + 14}px ${y + 18}px`;
  }
  const hideOp = () => { badge?.remove(); badge = null; };
  const linkable = (d: Drag, dest: string) => !inArchive(dest) && !inArchive(d.folder) && d.items.every((k) => !inArchive(k.at || ''));
  async function linkHere(d: Drag, dest: string) {
    try {
      for (const k of d.items) await tools.symlink(join(k.at || d.folder, k.name), join(dest, k.name));
      toast(plural(d.items.length, 'transfer.symlinked'), 'ok');
    } catch (e) {
      toast((e as Error).message, 'warn');
    }
  }

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
        return { el: row, dest };
      }
    }
    const tab = at('[data-tab]') as HTMLElement | null;
    if (tab && panelEl) {
      return { el: tab, dest: F.P(panel).tabs[+tab.dataset.tab!]!.path };
    }
    const go = at('[data-go]') as HTMLElement | null;
    if (go) return { el: go, dest: go.dataset.go! };
    const list = at('.list');
    if (list && (!d || panel !== d.from)) return { el: list, dest: F.T(panel).path };
    return null;
  }
  const clearDrop = () => document.querySelectorAll('.dragover, .drop').forEach((x) => x.classList.remove('dragover', 'drop'));
  function hover(tgt: Target | null) {
    if (!tgt) return clearDrop();
    if (!tgt.el.classList.contains('drop') && !tgt.el.classList.contains('dragover')) {
      clearDrop();
      tgt.el.classList.add(tgt.el.classList.contains('list') ? 'dragover' : 'drop');
    }
  }
  function endDrag() {
    if (drag && !drag.native) ended = { drag, at: Date.now() };
    drag = null;
    clearDrop();
    hideOp();
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
    drag = { from: F.active, items: F.sel(), folder: tab.path, op: dragOp(e, onMac(), false) };
    const onDisk = host.desktop && !inArchive(tab.path) && drag.items.every((k) => !inArchive(k.at || ''));
    // WebView2 drags only within the page, and a WebKitGTK drag cannot be read by GTK 4 programs such as Nautilus, so
    // there the system's own drag carries the files, inside the window as well, where the page sees it like any other
    // and drops it the usual way.
    if (onDisk && (host.os === 'windows' || host.os === 'linux')) {
      e.preventDefault();
      const d = drag;
      d.native = true;
      void Promise.resolve(run('dragOut', paths(d))).finally(() => { if (drag === d) endDrag(); });
      return;
    }
    e.dataTransfer!.effectAllowed = 'all';
    e.dataTransfer!.setData('text/plain', drag.items.map((x) => x.name).join('\n'));
    // Elsewhere the webview hands a URI list on to the system, where file managers take it as files.
    if (onDisk) e.dataTransfer!.setData('text/uri-list', paths(drag).map((p) => 'file://' + encodeURI(nativePath(p, false)).replace(/[?#]/g, encodeURIComponent)).join('\r\n'));
  });
  document.addEventListener('dragover', (e) => {
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
    if (!tgt || (!drag && !files)) return hideOp();
    e.preventDefault();
    // Files from another program are always taken as a copy there, so it never deletes them after a move: the app
    // moves them itself where it should.
    if (!drag) {
      e.dataTransfer!.dropEffect = 'copy';
      return;
    }
    drag.op = dragOp(e, onMac(), linkable(drag, tgt.dest));
    e.dataTransfer!.dropEffect = drag.op;
    showOp(drag.op, e.clientX, e.clientY);
  });
  document.addEventListener('dragend', endDrag);
  document.addEventListener('drop', (e) => {
    clearDrop();
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
    hideOp();
    if (!tgt) return;
    const op = dragOp(e, onMac(), linkable(d, tgt.dest));
    if (op === 'link') void linkHere(d, tgt.dest);
    else F.transfer(op, d.items, tgt.dest, d.folder);
  });

  // The window's side of a drop. On Windows the page passed it the files and gets back their paths. On Linux the
  // window takes every drop with files itself, the app's own drags too, and the page only learns where it landed.
  function dropped(list: string[], x: number, y: number) {
    clearDrop();
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
      if (tgt && own.op === 'link') void linkHere(own, tgt.dest);
      else if (tgt) F.transfer(own.op, own.items, tgt.dest, own.folder);
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
    handleDragLeave: () => { if (!drag) clearDrop(); },
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

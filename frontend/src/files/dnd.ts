// Dragging rows copies onto a folder, a tab, a piece of the path, the sidebar or the other panel; Shift moves.
// Holding still over a folder, tab or path piece opens it, so a drop can go deeper, as in Finder.
// Files dragged in from outside go to the upload.
import { run } from '@/core/actions';
import { prefs } from '@/core/prefs';
import { view } from '@/core/views';
import { usePanels } from '@/stores/panels';
import type { Entry } from './entries';
import { join } from './paths';

interface Target { el: Element; dest: string; open?: () => void }

export function installDragDrop() {
  const F = usePanels();
  let drag: { from: number; items: Entry[] } | null = null;
  let spring: { el: Element | null; timer: ReturnType<typeof setTimeout> | undefined } = { el: null, timer: undefined };

  const panelOf = (el: Element | null) => (el ? +(el as HTMLElement).dataset.panel! : F.active);
  function dropTarget(e: DragEvent): Target | null {
    const at = (sel: string) => (e.target as Element).closest?.(sel) ?? null;
    if (!drag) return null;
    const panelEl = at('.panel[data-panel]'), panel = panelOf(panelEl);
    const row = at('.row.dir') as HTMLElement | null;
    if (row && panelEl) {
      const k = F.entries(panel)[+row.dataset.idx!];
      if (k && !drag.items.some((x) => x.name === k.name && (x.at || '') === (k.at || ''))) {
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
    if (list && panel !== drag.from) return { el: list, dest: F.T(panel).path };
    return null;
  }
  const clearDrop = () => document.querySelectorAll('.dragover, .drop').forEach((x) => x.classList.remove('dragover', 'drop'));
  const stopSpring = () => {
    clearTimeout(spring.timer);
    spring = { el: null, timer: undefined };
  };

  document.addEventListener('dragstart', (e) => {
    const row = (e.target as Element).closest?.<HTMLElement>('.row');
    const panelEl = row?.closest<HTMLElement>('.panel[data-panel]');
    if (!row || !panelEl || view.id !== 'files') return;
    F.active = +panelEl.dataset.panel!;
    const tab = F.T(), k = F.entries()[+row.dataset.idx!];
    if (!k) return;
    if (!tab.marked.has(k.name)) tab.marked = new Set([k.name]);
    drag = { from: F.active, items: F.sel() };
    e.dataTransfer!.effectAllowed = 'copyMove';
    e.dataTransfer!.setData('text/plain', drag.items.map((x) => x.name).join('\n'));
  });
  document.addEventListener('dragover', (e) => {
    const list = (e.target as Element).closest?.('.list');
    if (!drag && [...(e.dataTransfer?.types || [])].includes('Files') && list) {
      e.preventDefault();
      clearDrop();
      list.classList.add('dragover');
      return;
    }
    const tgt = dropTarget(e);
    if (!tgt) return clearDrop();
    e.preventDefault();
    if (!tgt.el.classList.contains('drop') && !tgt.el.classList.contains('dragover')) {
      clearDrop();
      tgt.el.classList.add(tgt.el.classList.contains('list') ? 'dragover' : 'drop');
    }
    if (spring.el !== tgt.el) {
      clearTimeout(spring.timer);
      spring = { el: tgt.el, timer: tgt.open ? setTimeout(() => { tgt.open!(); spring.el = null; }, 900) : undefined };
    }
  });
  document.addEventListener('dragend', () => {
    drag = null;
    stopSpring();
    clearDrop();
  });
  document.addEventListener('drop', (e) => {
    clearDrop();
    stopSpring();
    const list = (e.target as Element).closest?.('.list');
    if (!drag && list && e.dataTransfer?.files.length) {
      e.preventDefault();
      F.active = panelOf(list.closest('.panel[data-panel]'));
      run('uploadFiles', [...e.dataTransfer.files], F.T().path);
      return;
    }
    if (!drag) return;
    e.preventDefault();
    const tgt = dropTarget(e), items = drag.items, from = F.T(drag.from).path;
    drag = null;
    if (tgt) F.transfer(e.shiftKey ? 'move' : 'copy', items, tgt.dest, from);
  });

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

// Explorer's way to pick several files with the mouse: a drag that starts on free space draws a frame, and every entry
// it touches is marked, added to the marks with Ctrl. In the row views the cells beside the name count as free space,
// since the rows fill the whole width; the name itself still drags the files.
import { reactive } from 'vue';

export interface Frame { x: number; y: number; w: number; h: number }
export interface BandHost {
  // The rows drawn in the list, with the index of their entry in data-idx.
  rows: () => HTMLElement[];
  // The names of the entries by index; an empty one, such as the row that goes up, is never marked.
  nameAt: (idx: number) => string;
  marked: () => Set<string>;
  mark: (names: Set<string>) => void;
}

export const freeSpace = (target: Element, rowView: boolean) => {
  if (target.closest('.lhead, .lgroup')) return false;
  const row = target.closest('.row');
  return !row || (rowView && !target.closest('.name'));
};

const touches = (a: DOMRect, f: Frame) => a.left < f.x + f.w && a.right > f.x && a.top < f.y + f.h && a.bottom > f.y;

export function useBand(host: BandHost) {
  const frame = reactive({ on: false, x: 0, y: 0, w: 0, h: 0 });
  let start: { x: number; y: number; base: Set<string> } | null = null;
  let drew = false;

  function down(e: PointerEvent, rowView: boolean) {
    drew = false;
    if (e.button !== 0 || e.pointerType === 'touch' || !freeSpace(e.target as Element, rowView)) return;
    start = { x: e.clientX, y: e.clientY, base: e.ctrlKey || e.metaKey ? new Set(host.marked()) : new Set() };
    addEventListener('pointermove', move);
    addEventListener('pointerup', up, { once: true });
  }

  function move(e: PointerEvent) {
    if (!start) return;
    if (!frame.on && Math.hypot(e.clientX - start.x, e.clientY - start.y) < 5) return;
    Object.assign(frame, { on: true, x: Math.min(start.x, e.clientX), y: Math.min(start.y, e.clientY), w: Math.abs(e.clientX - start.x), h: Math.abs(e.clientY - start.y) });
    const names = new Set(start.base);
    for (const row of host.rows()) {
      const name = host.nameAt(+row.dataset.idx!);
      if (name && touches(row.getBoundingClientRect(), frame)) names.add(name);
    }
    host.mark(names);
  }

  function up() {
    removeEventListener('pointermove', move);
    drew = frame.on;
    frame.on = false;
    start = null;
  }

  // The click that ends a frame must not drop the marks it just made.
  const drawn = () => {
    const was = drew;
    drew = false;
    return was;
  };
  // A press on the cells beside a name draws a frame instead of dragging the row.
  const holding = () => start !== null;

  return { frame, down, drawn, holding };
}

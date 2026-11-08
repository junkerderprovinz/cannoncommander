// Reordering by dragging, as GlimStone's design language describes it and BombVault and KnightLoader do it: the
// carried item lifts and follows the pointer, the others slide aside while it passes, and on release it slides into
// its gap. Escape or a cancelled pointer slides everything back and writes nothing. GlimStone's glim-drag-* classes
// draw the states from the --drag-* tokens; the code here sets only each item's `translate` and switches the classes.
import { dropIndex, type DropRect } from '@glimstone/core';
import { useDragArmed } from '@glimstone/vue';
import { computed, onBeforeUnmount, onMounted, ref, watch, type Ref } from 'vue';
import { quietTips } from '@/core/tips';

const LIFT = 'glim-drag-lift';
const SHIFT = 'glim-drag-shift';
const SETTLE = 'glim-drag-settle';

// How far a press travels before it becomes a drag: Windows' SM_CXDRAG, measured per axis.
const THRESHOLD_PX = 5;
// A finger arms a drag by holding this long, and moving further than the slop first makes it a scroll or a tap.
const HOLD_MS = 300;
const HOLD_SLOP_PX = 8;
// The token's scale suits a tile. A row as wide as a window would grow by dozens of pixels, so a wider item grows by
// the pixels one this wide does.
const LIFT_REF_PX = 360;

interface Point { x: number; y: number }

interface Lifted {
  id: string;
  down: Point;
  /** Its box when it lifted, which the pointer's travel is added to. */
  from: DropRect;
  /** The range its corner may take along the list, inside the area the items cover. */
  min: number;
  max: number;
  axis: 'x' | 'y';
  rtl: boolean;
  scale: number;
}

interface Press { id: string; down: Point; pointerId: number; hold: boolean }

export interface ReorderOptions {
  /** The resting order. */
  ids: () => readonly string[];
  /** The items' offsetParent, the layout a drag measures in. */
  list: Ref<HTMLElement | null>;
  /** The attribute carrying an item's id; items without it cannot be carried or passed. */
  attr: string;
  /** How a mouse arms the drag: `hold` where the item is also a click target, `move` where it is not. */
  arm: 'hold' | 'move';
  /** Called with the full new order and the carried item after a drop that changed it. */
  onReorder: (ids: string[], moved: string) => void;
}

const boxOf = (n: HTMLElement): DropRect => ({ left: n.offsetLeft, top: n.offsetTop, width: n.offsetWidth, height: n.offsetHeight });
const sameOrder = (a: readonly string[], b: readonly string[]) => a.length === b.length && a.every((id, i) => id === b[i]);
const clamp = (v: number, lo: number, hi: number) => Math.min(Math.max(v, lo), hi);

function liftScale(n: HTMLElement): number {
  const token = parseFloat(getComputedStyle(n).getPropertyValue('--drag-lift-scale')) || 1;
  return 1 + (token - 1) * Math.min(1, LIFT_REF_PX / Math.max(n.offsetWidth, 1));
}

function settleMs(n: HTMLElement): number {
  return Math.max(0, ...getComputedStyle(n).transitionDuration.split(',').map((s) => parseFloat(s) || 0)) * 1000;
}

/** Where a node is painted, a slide in flight included. */
function painted(n: HTMLElement): Point {
  const t = getComputedStyle(n).translate;
  const [x = 0, y = 0] = !t || t === 'none' ? [] : t.split(' ').map(parseFloat);
  return { x: n.offsetLeft + x, y: n.offsetTop + y };
}

/** useReorder is the drag gesture for a list of ids; see the file comment. */
export function useReorder({ ids, list, attr, arm, onReorder }: ReorderOptions) {
  const held = ref<string | null>(null);
  const settling = ref<string | null>(null);
  // The order laid out while an item is carried or landing. The pointer's travel goes straight to the carried item's
  // `translate` rather than through Vue, so it keeps up with the pointer.
  const live = ref<string[] | null>(null);
  let pressed: Press | null = null;
  let holdTimer = 0;
  let settleTimer = 0;
  let pointer: Point = { x: 0, y: 0 };
  let lifted: Lifted | null = null;
  // Where each item was painted before the live order moved it, for the slide.
  let slideFrom: Map<string, Point> | null = null;
  let swallow = false;

  useDragArmed(list, () => held.value !== null);
  // A bubble would stay behind where the carried item was, or point at one that is sliding away.
  watch(held, (id) => quietTips(id !== null));

  const nodes = () => Array.from(list.value?.querySelectorAll<HTMLElement>(`[${attr}]`) ?? []);
  const nodeOf = (id: string) => nodes().find((n) => n.getAttribute(attr) === id);

  function cancelPress() {
    clearTimeout(holdTimer);
    pressed = null;
  }

  function press(e: PointerEvent, id: string) {
    if (e.button !== 0 || lifted) return;
    cancelPress();
    const down = { x: e.clientX, y: e.clientY };
    const hold = arm === 'hold' || e.pointerType === 'touch';
    pressed = { id, down, pointerId: e.pointerId, hold };
    pointer = down;
    swallow = false;
    if (hold) holdTimer = window.setTimeout(() => lift(id, down), HOLD_MS);
  }

  function lift(id: string, down: Point) {
    const node = nodeOf(id);
    const el = list.value;
    if (!node || !el) return;
    const boxes = nodes().map(boxOf);
    const from = boxOf(node);
    // A list that turns into a row on a narrow screen is carried sideways.
    const axis = boxes.length > 1 && boxes.every((b) => b.top === from.top) ? 'x' : 'y';
    const scale = liftScale(node);
    const [start, size] = axis === 'x' ? (['left', 'width'] as const) : (['top', 'height'] as const);
    const grow = ((scale - 1) / 2) * from[size];
    lifted = {
      id, down, from, axis, scale,
      min: Math.min(...boxes.map((b) => b[start])) + grow,
      max: Math.max(...boxes.map((b) => b[start] + b[size])) - from[size] - grow,
      rtl: getComputedStyle(el).direction === 'rtl',
    };
    clearTimeout(settleTimer);
    settling.value = null;
    held.value = id;
    live.value = [...ids()];
  }

  /** The carried item's unscaled box for the current pointer, kept inside the list only where it is drawn. */
  function heldAt(h: Lifted, drawn: boolean): DropRect {
    const along = h.axis === 'x' ? h.from.left + pointer.x - h.down.x : h.from.top + pointer.y - h.down.y;
    const at = drawn ? clamp(along, h.min, h.max) : along;
    return h.axis === 'x' ? { ...h.from, left: at } : { ...h.from, top: at };
  }

  function placeHeld(h: Lifted) {
    const node = nodeOf(h.id);
    if (!node) return;
    node.style.scale = String(h.scale);
    const at = heldAt(h, true);
    node.style.translate = `${at.left - node.offsetLeft}px ${at.top - node.offsetTop}px`;
  }

  /** The order the carried item's place asks for, measured in the layout the live order gives the list. */
  function aim(h: Lifted): string[] {
    const node = nodeOf(h.id);
    if (!node) return live.value ?? [...ids()];
    const others = nodes().filter((n) => n !== node);
    const order = others.map((n) => n.getAttribute(attr)!);
    order.splice(dropIndex(heldAt(h, false), boxOf(node), others.map(boxOf), { axis: h.axis, rtl: h.rtl }), 0, h.id);
    return order;
  }

  function release(h: Lifted) {
    lifted = null;
    // The click that follows the release would open the carried item.
    swallow = true;
    const node = nodeOf(h.id);
    if (node) node.style.scale = '';
    held.value = null;
    settling.value = h.id;
  }

  function putBack(h: Lifted) {
    cancelPress();
    const order = live.value;
    if (order && !sameOrder(order, ids())) {
      slideFrom = new Map(nodes().map((n) => [n.getAttribute(attr)!, painted(n)]));
      live.value = [...ids()];
    }
    release(h);
  }

  // Once the live order has moved the items in the DOM, the carried one is placed under the pointer again and every
  // other one that moved starts from where it was painted and slides to its new slot.
  watch(live, () => {
    const h = lifted;
    if (h) placeHeld(h);
    const from = slideFrom;
    slideFrom = null;
    if (!from) return;
    const moved: HTMLElement[] = [];
    for (const n of nodes()) {
      const id = n.getAttribute(attr)!;
      const was = from.get(id);
      if (!was || id === h?.id) continue;
      const dx = was.x - n.offsetLeft;
      const dy = was.y - n.offsetTop;
      if (dx === 0 && dy === 0) continue;
      n.style.transition = 'none';
      n.style.translate = `${dx}px ${dy}px`;
      moved.push(n);
    }
    if (!moved.length) return;
    // Reading layout commits the jump back, so the transition starts from there.
    void list.value?.offsetWidth;
    for (const n of moved) {
      n.style.transition = '';
      n.style.translate = '0px 0px';
    }
  }, { flush: 'post' });

  // The dropped item slides into its slot, and once the landing has played the offsets and the live order go.
  watch(settling, (id) => {
    if (!id) return;
    const node = nodeOf(id);
    if (node) node.style.translate = '0px 0px';
    settleTimer = window.setTimeout(() => {
      for (const n of nodes()) n.style.translate = '';
      settling.value = null;
      live.value = null;
    }, node ? settleMs(node) : 0);
  }, { flush: 'post' });

  function onMove(e: PointerEvent) {
    const p = pressed;
    if (!p || e.pointerId !== p.pointerId) return;
    pointer = { x: e.clientX, y: e.clientY };
    if (!lifted) {
      const dx = Math.abs(e.clientX - p.down.x);
      const dy = Math.abs(e.clientY - p.down.y);
      if (p.hold) {
        if (dx > HOLD_SLOP_PX || dy > HOLD_SLOP_PX) cancelPress();
        return;
      }
      if (dx < THRESHOLD_PX && dy < THRESHOLD_PX) return;
      lift(p.id, p.down);
    }
    const h = lifted;
    if (!h) return;
    placeHeld(h);
    const next = aim(h);
    if (live.value && !sameOrder(next, live.value)) {
      slideFrom = new Map(nodes().map((n) => [n.getAttribute(attr)!, painted(n)]));
      live.value = next;
    }
  }

  function onUp(e: PointerEvent) {
    if (e.pointerId !== pressed?.pointerId) return;
    cancelPress();
    const h = lifted;
    if (!h) return;
    const order = live.value;
    // A drop where it started is not a change and writes nothing.
    if (order && !sameOrder(order, ids())) onReorder(order, h.id);
    release(h);
  }

  function onCancel(e: PointerEvent) {
    if (e.pointerId !== pressed?.pointerId) return;
    if (lifted) putBack(lifted);
    else cancelPress();
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape' && lifted) putBack(lifted);
  }

  // Once armed the gesture is the drag's: a long press opens no context menu, and a moving finger drags instead of
  // scrolling.
  function onTouch(e: Event) {
    if (lifted) e.preventDefault();
  }

  onMounted(() => {
    document.addEventListener('pointermove', onMove);
    document.addEventListener('pointerup', onUp);
    document.addEventListener('pointercancel', onCancel);
    document.addEventListener('keydown', onKey);
    document.addEventListener('contextmenu', onTouch);
    document.addEventListener('touchmove', onTouch, { passive: false });
  });
  onBeforeUnmount(() => {
    cancelPress();
    clearTimeout(settleTimer);
    document.removeEventListener('pointermove', onMove);
    document.removeEventListener('pointerup', onUp);
    document.removeEventListener('pointercancel', onCancel);
    document.removeEventListener('keydown', onKey);
    document.removeEventListener('contextmenu', onTouch);
    document.removeEventListener('touchmove', onTouch);
  });

  // Items that arrive while one is carried are drawn after the live order rather than left out until it ends.
  const order = computed(() => {
    const now = ids();
    const l = live.value;
    return l ? [...l.filter((id) => now.includes(id)), ...now.filter((id) => !l.includes(id))] : [...now];
  });

  function look(id: string): string {
    if (id === held.value) return LIFT;
    if (id === settling.value) return SETTLE;
    return held.value !== null || settling.value !== null ? SHIFT : '';
  }

  /** Whether a click is the release of a drag and has to be swallowed. A key or a screen reader clicks with a detail of 0. */
  function endsDrag(e: MouseEvent): boolean {
    const ends = swallow && e.detail > 0;
    swallow = false;
    return ends;
  }

  return { order, held, press, look, endsDrag };
}

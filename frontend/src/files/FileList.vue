<script setup lang="ts">
// The list of one panel in whichever view its folder has. A long folder draws only the lines in sight plus a margin,
// with spacers for the rest, so ten thousand files scroll like twenty.
import { EmptyState } from '@glimstone/vue';
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue';
import { G, glyphComponent } from '@/core/glyphs';
import { now } from '@/core/format';
import { dialMs, moving } from '@/core/motion';
import { prefs, setPref } from '@/core/prefs';
import { t } from '@/i18n';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';
import { COL, columnLabel, Columns } from './columns';
import { useBand } from './band';
import { type Entry, groupHeads } from './entries';
import { columnMenu, listMenu, rowMenu } from './menus';
import ListItem from './ListItem.vue';
import Miller from './Miller.vue';
import { join } from './paths';
import { CARD_VIEWS, ROW_VIEWS } from './views';
import { CARD_STEPS, shownTile, thumbSize, wheelZoom, type Zoom } from './zoom';

const props = defineProps<{ i: number }>();
const F = usePanels();
const listing = useListing();

const tab = computed(() => F.T(props.i));
const list = computed(() => F.entries(props.i));
const v = computed(() => F.viewOf(tab.value));
const grid = computed(() => CARD_VIEWS.includes(v.value));
const tile = computed(() => (CARD_STEPS.some(([id]) => id === v.value) ? F.tileOf(tab.value) : undefined));
const thumb = computed(() => thumbSize(tile.value, devicePixelRatio));
const visible = computed(() => Columns.visible());
const group = computed(() => F.groupOf(tab.value));
const heads = computed(() => groupHeads(list.value, group.value, now()));
const groupName = (name: string) => t(`groups.${name}`);
const el = ref<HTMLElement | null>(null);

// Lines are what scrolls past: a heading, or as many entries as fit across. The list view flows sideways and the
// column view has its own columns, so both draw everything.
const VIRTUAL_FROM = 300;
const size = reactive({ per: 1, line: 0, head: 0, gap: 0, top: 0, height: 0 });
const virtual = computed(() => list.value.length > VIRTUAL_FROM && !['list', 'columns'].includes(v.value));
interface Line { head?: number; from: number; to: number; y: number; h: number }
const lines = computed<Line[]>(() => {
  const out: Line[] = [];
  const line = size.line || 32, head = size.head || line, per = grid.value ? size.per : 1;
  let y = 0, k = 0;
  const n = list.value.length;
  while (k < n) {
    if (heads.value[k]) { out.push({ head: k, from: k, to: k, y, h: head }); y += head; }
    let end = k + 1;
    while (end < n && end - k < per && !heads.value[end]) end++;
    out.push({ from: k, to: end, y, h: line });
    y += line;
    k = end;
  }
  return out;
});
const range = computed(() => {
  const all = lines.value;
  if (!virtual.value || !size.line) return { a: 0, b: Math.min(all.length, 60), before: 0, after: all.length > 60 ? all.at(-1)!.y + all.at(-1)!.h - (all[60]?.y ?? 0) : 0 };
  const margin = size.height;
  let a = all.findIndex((l) => l.y + l.h >= size.top - margin);
  if (a < 0) a = all.length;
  let b = a;
  while (b < all.length && all[b]!.y <= size.top + size.height + margin) b++;
  const end = all.length ? all.at(-1)!.y + all.at(-1)!.h : 0;
  return { a, b, before: all[a]?.y ?? end, after: b < all.length ? end - all[b]!.y : 0 };
});
interface Drawn { head?: { name: string; count: number }; k?: Entry; idx: number }
const drawn = computed<Drawn[]>(() => {
  if (!virtual.value) return list.value.flatMap((k, idx) => (heads.value[idx] ? [{ head: heads.value[idx]!, idx: -idx - 1 }, { k, idx }] : [{ k, idx }]));
  const out: Drawn[] = [];
  for (const l of lines.value.slice(range.value.a, range.value.b)) {
    if (l.head !== undefined) out.push({ head: heads.value[l.head]!, idx: -l.head - 1 });
    else for (let idx = l.from; idx < l.to; idx++) out.push({ k: list.value[idx]!, idx });
  }
  return out;
});

// Sizes are read off the drawn lines, so density, font and window width all count.
function measure() {
  const l = el.value;
  if (!l) return;
  size.top = l.scrollTop;
  size.height = l.clientHeight;
  const rows = [...l.querySelectorAll<HTMLElement>(':scope > .row')];
  if (!rows.length) return;
  const first = rows[0]!;
  const across = rows.filter((r) => r.offsetTop === first.offsetTop).length;
  const next = rows[across];
  size.gap = grid.value ? parseFloat(getComputedStyle(l).rowGap) || 0 : 0;
  size.per = Math.max(1, across);
  size.line = next && next.offsetTop > first.offsetTop ? next.offsetTop - first.offsetTop : first.offsetHeight + size.gap;
  const h = l.querySelector<HTMLElement>(':scope > .lgroup');
  if (h) size.head = h.offsetHeight + size.gap;
}
// A spacer is a grid item too, so the gap after it is already part of the distance it stands for.
const pad = (px: number) => Math.max(0, px - size.gap) + 'px';
let observer: ResizeObserver | undefined;
onMounted(() => {
  measure();
  observer = new ResizeObserver(measure);
  if (el.value) observer.observe(el.value);
  if (v.value === 'columns' && el.value) el.value.scrollLeft = el.value.scrollWidth;
});
onBeforeUnmount(() => observer?.disconnect());
watch([v, tile, () => list.value.length, () => prefs.density], () => nextTick(() => {
  measure();
  if (v.value === 'columns' && el.value) el.value.scrollLeft = el.value.scrollWidth;
}));
const onScroll = () => { if (virtual.value && el.value) size.top = el.value.scrollTop; };

// Ctrl or Cmd and the wheel zoom this panel only, and only over its files; anywhere else the browser keeps its own
// page zoom. The entry under the pointer stays under it while the cards grow, or the cursor row where it was.
interface Anchor { idx: number; fx: number; fy: number; x: number; y: number }
function anchorOf(e: WheelEvent): Anchor | null {
  const l = el.value!, over = (e.target as Element).closest<HTMLElement>('.row');
  const row = over ?? l.querySelector<HTMLElement>(`.row[data-idx="${tab.value.cursor}"]`);
  if (!row) return null;
  const box = row.getBoundingClientRect(), at = l.getBoundingClientRect();
  return over
    ? { idx: +row.dataset.idx!, fx: (e.clientX - box.left) / box.width, fy: (e.clientY - box.top) / box.height, x: e.clientX - at.left, y: e.clientY - at.top }
    : { idx: +row.dataset.idx!, fx: 0, fy: 0, x: box.left - at.left, y: box.top - at.top };
}
function place(a: Anchor) {
  const l = el.value;
  if (!l) return;
  measure();
  const row = l.querySelector<HTMLElement>(`.row[data-idx="${a.idx}"]`), at = l.getBoundingClientRect();
  if (row) {
    const box = row.getBoundingClientRect();
    l.scrollTop += box.top + a.fy * box.height - at.top - a.y;
    l.scrollLeft += box.left + a.fx * box.width - at.left - a.x;
  } else {
    const line = lines.value.find((x) => x.head === undefined && a.idx >= x.from && a.idx < x.to);
    if (line) l.scrollTop = line.y - a.y;
  }
  size.top = l.scrollTop;
}
// The cards grow over the length of the dial, so the anchor is put back on every frame until they stand.
let holding = 0;
function hold(a: Anchor) {
  const turn = ++holding, until = performance.now() + dialMs('--motion-zoom-dur');
  const step = () => {
    if (turn !== holding) return;
    place(a);
    if (performance.now() < until) requestAnimationFrame(step);
  };
  nextTick(step);
}
const zoom = wheelZoom(() => ({ view: v.value, tile: tile.value }), (z: Zoom) => {
  const swap = z.view !== v.value;
  F.setZoom(tab.value, z, false);
  const ms = dialMs('--motion-zoom-dur');
  if (swap && moving() && ms) nextTick(() => el.value?.animate([{ opacity: 0.35, offset: 0 }], { duration: ms, easing: 'ease-out' }));
});
function onWheel(e: WheelEvent) {
  if (!el.value || !(e.ctrlKey || e.metaKey)) return;
  const a = anchorOf(e);
  if (!zoom(e)) return;
  e.preventDefault();
  if (a) hold(a);
}
onMounted(() => el.value?.addEventListener('wheel', onWheel, { passive: false }));
onBeforeUnmount(() => el.value?.removeEventListener('wheel', onWheel));

// The rows of a folder just opened come in one after another, on GlimStone's stagger dials. A key or a press ends it
// at once, so the entrance never stands between a key and the row it moves to.
const entering = ref(false);
let entered: ReturnType<typeof setTimeout> | undefined;
function settle() {
  entering.value = false;
  clearTimeout(entered);
  removeEventListener('keydown', settle, true);
  removeEventListener('pointerdown', settle, true);
}
let arrived = false;
watch(() => F.loaded(props.i) && list.value.length > 0, (ready) => {
  if (!ready || arrived) return;
  arrived = true;
  const lasts = dialMs('--motion-stagger-cap') + dialMs('--motion-stagger-dur');
  if (!moving() || !lasts) return;
  entering.value = true;
  entered = setTimeout(settle, lasts);
  addEventListener('keydown', settle, true);
  addEventListener('pointerdown', settle, true);
}, { immediate: true });
onBeforeUnmount(settle);

// The cursor stays in sight after a key moved it, also onto a line that is not drawn yet.
watch(() => [tab.value.cursor, props.i === F.active] as const, async ([cursor, active]) => {
  if (!active) return;
  await nextTick();
  const l = el.value;
  const row = l?.querySelector<HTMLElement>(`.row[data-idx="${cursor}"]`);
  if (row) {
    row.scrollIntoView({ block: 'nearest' });
    return;
  }
  const line = lines.value.find((x) => x.head === undefined && cursor >= x.from && cursor < x.to);
  if (l && line) l.scrollTop = Math.max(0, line.y - l.clientHeight / 2);
});

// The rows on screen and the columns shown decide what fs/meta is asked for; sorting by such a column needs all rows.
const metaCols = computed(() => {
  const extra = v.value === 'content' ? ['dimensions', 'duration', 'pages'] : grid.value ? ['duration'] : [];
  return ROW_VIEWS.includes(v.value) ? Columns.metaWanted(tab.value.sort, extra) : [...new Set([...extra, tab.value.sort])].filter((id) => COL[id]?.meta);
});
watch([() => drawn.value, metaCols], () => {
  if (!metaCols.value.length) return;
  const rows = COL[tab.value.sort]?.meta ? list.value : drawn.value.flatMap((d) => (d.k ? [d.k] : []));
  listing.wantMeta(rows.map((k) => join(k.at || tab.value.path, k.name)), metaCols.value);
}, { immediate: true });

function sortBy(key: string) {
  const tb = tab.value;
  if (tb.sort === key) tb.dir *= -1;
  else Object.assign(tb, { sort: key, dir: 1 });
}

// With the selection as in Explorer, a drag on free space draws a frame; the column view picks in its own columns.
const band = useBand({
  rows: () => [...(el.value?.querySelectorAll<HTMLElement>(':scope > .row') ?? [])],
  nameAt: (idx) => (list.value[idx] && !list.value[idx]!.up ? list.value[idx]!.name : ''),
  marked: () => tab.value.marked,
  mark: (names) => { tab.value.marked = names; },
});
const banding = () => prefs.clickSelect && v.value !== 'columns';
const onDown = (e: PointerEvent) => { if (banding()) band.down(e, ROW_VIEWS.includes(v.value)); };
const onDragStart = (e: DragEvent) => {
  if (!band.holding()) return;
  e.preventDefault();
  e.stopPropagation();
};

// Two clicks are paired here instead of by dblclick, so a Shift or Ctrl click never opens anything. As in a
// commander, a click only moves the cursor and Ctrl or Shift mark; as in Explorer, a click marks the entry alone.
let lastClick: { idx?: number; panel?: number; at?: number } = {};
function onClick(e: MouseEvent) {
  if (band.drawn()) return;
  const target = e.target as Element;
  const row = target.closest<HTMLElement>('.row');
  const tb = tab.value;
  if (row) {
    const idx = +row.dataset.idx!, l = list.value;
    if (e.shiftKey) {
      const [a, b] = [Math.min(tb.cursor, idx), Math.max(tb.cursor, idx)];
      if (prefs.clickSelect) tb.marked = new Set();
      for (let k = a; k <= b; k++) if (!l[k]!.up) tb.marked.add(l[k]!.name);
    } else if (e.ctrlKey || e.metaKey) {
      if (!l[idx]!.up) tb.marked.has(l[idx]!.name) ? tb.marked.delete(l[idx]!.name) : tb.marked.add(l[idx]!.name);
    } else if (prefs.clickSelect) {
      tb.marked = l[idx]!.up ? new Set() : new Set([l[idx]!.name]);
    }
    const again = lastClick.idx === idx && lastClick.panel === props.i && e.timeStamp - lastClick.at! < 450 && !e.shiftKey && !e.ctrlKey;
    lastClick = { idx, panel: props.i, at: e.timeStamp };
    tb.cursor = idx;
    if (again) {
      lastClick = {};
      F.open(props.i, l[idx]!);
    }
    return;
  }
  const go = target.closest<HTMLElement>('.mitem[data-go]');
  if (go) {
    F.go(go.dataset.go!, props.i);
    return;
  }
  // A click on empty space in the list drops the selection, as in Explorer.
  if (!target.closest('.lhead') && tb.marked.size) tb.marked = new Set();
}

function onMenu(e: MouseEvent) {
  e.preventDefault();
  F.active = props.i;
  if ((e.target as Element).closest('.lhead')) return columnMenu(e.clientX, e.clientY);
  const row = (e.target as Element).closest<HTMLElement>('.row');
  if (!row) return listMenu(e.clientX, e.clientY, e.shiftKey);
  const tb = tab.value, idx = +row.dataset.idx!, k = list.value[idx]!;
  if (!tb.marked.has(k.name)) {
    tb.marked = new Set();
    tb.cursor = idx;
  }
  rowMenu(e.clientX, e.clientY, e.shiftKey);
}

// The grip on a header's right edge sets that column's width, kept in rem like the defaults. A header moves live
// under the pointer; a press without movement stays a click that sorts.
const REM = () => parseFloat(getComputedStyle(document.documentElement).fontSize);
const colSize = { id: '', x: 0, w: 0, moved: false };
const colDrag = reactive({ id: '', x: 0, moved: false });
function onHeadDown(e: PointerEvent) {
  if (e.button !== 0) return;
  const target = e.target as Element;
  const grip = target.closest<HTMLElement>('[data-grip]');
  if (grip) {
    e.preventDefault();
    Object.assign(colSize, { id: grip.dataset.grip!, x: e.clientX, w: grip.closest('.lh')!.getBoundingClientRect().width / REM() - 0.7, moved: false });
    document.documentElement.classList.add('col-sizing');
  } else {
    const h = target.closest<HTMLElement>('.lh');
    if (h) Object.assign(colDrag, { id: h.dataset.colid!, x: e.clientX, moved: false });
  }
  addEventListener('pointermove', onHeadMove);
  addEventListener('pointerup', onHeadUp, { once: true });
}
function onHeadMove(e: PointerEvent) {
  if (colSize.id) {
    if (!colSize.moved && Math.abs(e.clientX - colSize.x) < 3) return;
    colSize.moved = true;
    prefs.colWidths = { ...prefs.colWidths, [colSize.id]: Math.max(2.5, Math.round((colSize.w + (e.clientX - colSize.x) * (document.documentElement.dir === 'rtl' ? -1 : 1) / REM()) * 4) / 4) };
    return;
  }
  if (!colDrag.id || (!colDrag.moved && Math.abs(e.clientX - colDrag.x) < 6)) return;
  colDrag.moved = true;
  const over = [...(el.value?.querySelectorAll<HTMLElement>('.lh') ?? [])].find((h) => {
    const r = h.getBoundingClientRect();
    return e.clientX >= r.left && e.clientX <= r.right;
  });
  const order = Columns.shown();
  if (over && over.dataset.colid !== colDrag.id) {
    const from = order.indexOf(colDrag.id), to = order.indexOf(over.dataset.colid!);
    order.splice(to, 0, order.splice(from, 1)[0]!);
    prefs.columns = order;
  }
}
function onHeadUp() {
  removeEventListener('pointermove', onHeadMove);
  if (colSize.id) {
    document.documentElement.classList.remove('col-sizing');
    if (colSize.moved) setPref('colWidths', prefs.colWidths);
    colSize.id = '';
  }
  if (colDrag.id) {
    if (colDrag.moved) setPref('columns', Columns.shown());
    // The click that follows a drag must not sort.
    setTimeout(() => { colDrag.id = ''; colDrag.moved = false; }, 0);
  }
}
function onHeadClick(e: MouseEvent, key: string) {
  if ((e.target as Element).closest('[data-grip]') || colDrag.moved) return;
  sortBy(key);
}
// A double click on the grip fits the column to its widest cell.
function fit(id: string) {
  const n = visible.value.indexOf(id) + 1;
  const cells = [...(el.value?.querySelectorAll<HTMLElement>(`.row > :nth-child(${n})`) ?? [])];
  const label = el.value?.querySelector<HTMLElement>(`.lh[data-colid="${id}"]`);
  const widest = Math.max(label?.scrollWidth ?? 0, ...cells.map((c) => c.scrollWidth));
  setPref('colWidths', { ...prefs.colWidths, [id]: Math.max(2.5, Math.ceil((widest / REM() + 0.3) * 4) / 4) });
}
const cutOf = (k: Entry) => !!F.clip?.cut && F.clip.from === tab.value.path && F.clip.items.includes(k.name);
</script>

<template>
  <div ref="el" class="list" :class="[`view-${v}`, { 'grid-view': grid, zoomed: tile !== undefined }]" tabindex="0"
    :style="{ '--cols': Columns.template(), '--minw': Columns.minWidth(), '--zoom-tile': shownTile(v, tile) + 'px' }"
    @click="onClick" @contextmenu="onMenu" @scroll.passive="onScroll" @pointerdown="onDown" @dragstart.capture="onDragStart">
    <div v-if="ROW_VIEWS.includes(v)" class="lhead" data-lhead @pointerdown="onHeadDown">
      <button v-for="id in visible" :key="id" class="lh" :class="{ r: COL[id]!.r, sorted: tab.sort === Columns.sortKey(COL[id]!), moving: colDrag.moved && colDrag.id === id }"
        :data-sort="Columns.sortKey(COL[id]!)" :data-colid="id" @click="onHeadClick($event, Columns.sortKey(COL[id]!))">{{ columnLabel(id) }}<G v-if="tab.sort === Columns.sortKey(COL[id]!)" :n="tab.dir > 0 ? 'IconMoveUp' : 'IconMoveDown'" /><span v-if="!COL[id]!.fixed" class="lh-grip" :data-grip="id" aria-hidden="true" @dblclick.stop="fit(id)"></span></button>
    </div>
    <Miller v-if="v === 'columns'" :i="i" :list="list" />
    <template v-else>
      <div v-if="virtual && range.before" class="vpad" aria-hidden="true" :style="{ height: pad(range.before) }"></div>
      <template v-for="(d, n) in drawn" :key="d.k ? (d.k.at || '') + d.k.name : d.idx">
        <div v-if="d.head" class="lgroup"><span>{{ groupName(d.head.name) }}</span><span class="glim-num">{{ d.head.count }}</span></div>
        <ListItem v-else :k="d.k!" :idx="d.idx" :view="v" :path="tab.path" :marked="tab.marked.has(d.k!.name)" :cursor="d.idx === tab.cursor" :cut="cutOf(d.k!)" :mark="F.marks[i]?.[d.k!.name]" :thumb="thumb"
          :class="{ 'glim-stagger-row': entering && !cutOf(d.k!) }" :style="entering ? { '--row-i': n } : undefined" />
      </template>
      <div v-if="virtual && range.after" class="vpad" aria-hidden="true" :style="{ height: pad(range.after) }"></div>
    </template>
    <div v-if="band.frame.on" class="band" aria-hidden="true" :style="{ left: band.frame.x + 'px', top: band.frame.y + 'px', width: band.frame.w + 'px', height: band.frame.h + 'px' }"></div>
    <EmptyState v-if="!list.length && F.loaded(i)" nested :glyph="glyphComponent(tab.filter ? 'IconSearch' : 'IconFolder')" :title="tab.filter ? t('files.nothingFound', { q: tab.filter }) : t('files.empty')" :hint="tab.filter ? (tab.deep === false ? t('files.searchedHere') : t('files.searchedDeep')) : t('files.emptyHint')" />
  </div>
</template>

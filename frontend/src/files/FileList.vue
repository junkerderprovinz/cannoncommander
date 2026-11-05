<script setup lang="ts">
// The list of one panel in whichever view its folder has. A long folder draws only the lines in sight plus a margin,
// with spacers for the rest, so ten thousand files scroll like twenty.
import { EmptyState } from '@glimstone/vue';
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue';
import { G, glyphComponent } from '@/core/glyphs';
import { now } from '@/core/format';
import { prefs, setPref } from '@/core/prefs';
import { t } from '@/i18n';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';
import { COL, columnLabel, Columns } from './columns';
import { type Entry, groupHeads } from './entries';
import { columnMenu, listMenu, rowMenu } from './menus';
import ListItem from './ListItem.vue';
import Miller from './Miller.vue';
import { join } from './paths';
import { CARD_VIEWS, ROW_VIEWS } from './views';

const props = defineProps<{ i: number }>();
const F = usePanels();
const listing = useListing();

const tab = computed(() => F.T(props.i));
const list = computed(() => F.entries(props.i));
const v = computed(() => F.viewOf(tab.value));
const grid = computed(() => CARD_VIEWS.includes(v.value));
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
watch([v, () => list.value.length, () => prefs.density], () => nextTick(() => {
  measure();
  if (v.value === 'columns' && el.value) el.value.scrollLeft = el.value.scrollWidth;
}));
const onScroll = () => { if (virtual.value && el.value) size.top = el.value.scrollTop; };

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

// Two clicks are paired here instead of by dblclick, so a Shift or Ctrl click never opens anything.
let lastClick: { idx?: number; panel?: number; at?: number } = {};
function onClick(e: MouseEvent) {
  const target = e.target as Element;
  const row = target.closest<HTMLElement>('.row');
  const tb = tab.value;
  if (row) {
    const idx = +row.dataset.idx!, l = list.value;
    if (e.shiftKey) {
      const [a, b] = [Math.min(tb.cursor, idx), Math.max(tb.cursor, idx)];
      for (let k = a; k <= b; k++) if (!l[k]!.up) tb.marked.add(l[k]!.name);
    } else if (e.ctrlKey || e.metaKey) {
      if (!l[idx]!.up) tb.marked.has(l[idx]!.name) ? tb.marked.delete(l[idx]!.name) : tb.marked.add(l[idx]!.name);
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
  if (!row) return listMenu(e.clientX, e.clientY);
  const tb = tab.value, idx = +row.dataset.idx!, k = list.value[idx]!;
  if (!tb.marked.has(k.name)) {
    tb.marked = new Set();
    tb.cursor = idx;
  }
  rowMenu(e.clientX, e.clientY);
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
    prefs.colWidths = { ...prefs.colWidths, [colSize.id]: Math.max(2.5, Math.round((colSize.w + (e.clientX - colSize.x) / REM()) * 4) / 4) };
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
  <div ref="el" class="list" :class="[`view-${v}`, { 'grid-view': grid }]" tabindex="0" :style="{ '--cols': Columns.template(), '--minw': Columns.minWidth() }"
    @click="onClick" @contextmenu="onMenu" @scroll.passive="onScroll">
    <div v-if="ROW_VIEWS.includes(v)" class="lhead" data-lhead @pointerdown="onHeadDown">
      <button v-for="id in visible" :key="id" class="lh" :class="{ r: COL[id]!.r, sorted: tab.sort === Columns.sortKey(COL[id]!), moving: colDrag.moved && colDrag.id === id }"
        :data-sort="Columns.sortKey(COL[id]!)" :data-colid="id" @click="onHeadClick($event, Columns.sortKey(COL[id]!))">{{ columnLabel(id) }}<G v-if="tab.sort === Columns.sortKey(COL[id]!)" :n="tab.dir > 0 ? 'IconMoveUp' : 'IconMoveDown'" /><span v-if="!COL[id]!.fixed" class="lh-grip" :data-grip="id" aria-hidden="true" @dblclick.stop="fit(id)"></span></button>
    </div>
    <Miller v-if="v === 'columns'" :i="i" :list="list" />
    <template v-else>
      <div v-if="virtual && range.before" class="vpad" aria-hidden="true" :style="{ height: pad(range.before) }"></div>
      <template v-for="d in drawn" :key="d.k ? (d.k.at || '') + d.k.name : d.idx">
        <div v-if="d.head" class="lgroup"><span>{{ groupName(d.head.name) }}</span><span class="glim-num">{{ d.head.count }}</span></div>
        <ListItem v-else :k="d.k!" :idx="d.idx" :view="v" :path="tab.path" :marked="tab.marked.has(d.k!.name)" :cursor="d.idx === tab.cursor" :cut="cutOf(d.k!)" :mark="F.marks[i]?.[d.k!.name]" />
      </template>
      <div v-if="virtual && range.after" class="vpad" aria-hidden="true" :style="{ height: pad(range.after) }"></div>
    </template>
    <EmptyState v-if="!list.length && F.loaded(i)" nested :glyph="glyphComponent(tab.filter ? 'IconSearch' : 'IconFolder')" :title="tab.filter ? t('files.nothingFound', { q: tab.filter }) : t('files.empty')" :hint="tab.filter ? (tab.deep === false ? t('files.searchedHere') : t('files.searchedDeep')) : t('files.emptyHint')" />
  </div>
</template>

<script setup lang="ts">
// Speicherbelegung like TreeSize: one folder as a tree sorted by size, as a treemap, by file type and as its largest
// files. Whatever it lists can be picked and deleted right here. The numbers come from tools/diskusage, one folder
// at a time, so opening a branch of the tree asks for that folder.
import { EmptyState, IconButton, Meter, Selector, SubCard } from '@glimstone/vue';
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue';
import { type DiskUsage, tools } from '@/api/tools';
import { fmtDate, fmtSize } from '@/core/format';
import { G, glyphOf, glyphComponent } from '@/core/glyphs';
import { menu } from '@/core/menu';
import { prefs } from '@/core/prefs';
import { toast } from '@/core/toast';
import { go as goView, view } from '@/core/views';
import type { WinEntry } from '@/core/windows';
import { toEntry } from '@/files/entries';
import { inArchive, join, parent } from '@/files/paths';
import { t } from '@/i18n';
import { deleteItems } from '@/features/trash/trash';
import { usePanels } from '@/stores/panels';
import Crumbs from '@/ui/Crumbs.vue';
import { copyText, folderSteps, num, plural, setFoot, segItems } from '@/ui/kit';
import { ended } from './ended';
import { fit, outermost, pct, squarify } from './treemap';

type View = 'tree' | 'map' | 'types' | 'largest';
type SortKey = 'name' | 'size' | 'share' | 'files' | 'dirs' | 'time';
interface Row { name: string; dir: boolean; at: string; path: string; size: number; files: number; dirs: number; time: number; depth: number; whole: number; canOpen: boolean }

const props = defineProps<{ start: string; win: WinEntry; close: () => void }>();
const F = usePanels();
const cur = ref(props.start);
const mode = ref<View>('map');
const sort = reactive({ by: 'size' as SortKey, dir: -1 });
const kind = ref<{ kind: string; name: string; glyph: string } | null>(null);
const open = reactive(new Set<string>());
const sel = reactive(new Map<string, Row>());
const focus = ref<string | null>(null);
let anchor: string | null = null;
const body = ref<HTMLElement | null>(null);
const box = reactive({ w: 0, h: 0 });

const VIEWS = computed(() => ([['tree', 'IconRowMedium'], ['map', 'IconModules'], ['types', 'IconFiles'], ['largest', 'IconSort']] as const).map(([v, g]) => [v, t(`diskusage.views.${v}`), g] as const));
const COLS: SortKey[] = ['name', 'size', 'share', 'files', 'dirs', 'time'];
const KIND_GLYPH: Record<string, string> = { img: 'IconFileImage', video: 'IconFileVideo', audio: 'IconFileAudio', arch: 'IconArchive', disk: 'IconDatabase', doc: 'IconFileText', text: 'IconFileText', code: 'IconCode', file: 'IconFile' };

// What the server measured, by folder, and the largest files of one kind, by folder and kind.
const usage = reactive(new Map<string, DiskUsage>());
const largestOf = reactive(new Map<string, DiskUsage['largest']>());
const asking = new Set<string>();
function measure(path: string, again = false) {
  if ((usage.has(path) && !again) || asking.has(path)) return;
  asking.add(path);
  tools.diskUsage(path, { hidden: !!prefs.showHidden }).then((u) => usage.set(path, u), (e) => { if (path === cur.value) toast((e as Error).message, 'warn'); })
    .finally(() => asking.delete(path));
}
function largestKind(path: string, k: string) {
  const key = path + '\n' + k;
  if (largestOf.has(key) || asking.has(key)) return largestOf.get(key);
  asking.add(key);
  tools.diskUsage(path, { hidden: !!prefs.showHidden, kind: k }).then((u) => largestOf.set(key, u.largest), () => {}).finally(() => asking.delete(key));
  return undefined;
}
const here = computed(() => usage.get(cur.value));
watch(cur, (p) => measure(p), { immediate: true });

// Deleting, or taking a deletion back, changes the sizes; every job that ends has them measured again. The old numbers
// stay on screen until the new ones are there.
watch(ended, () => {
  for (const p of [...usage.keys()]) if (p !== cur.value && !open.has(p)) usage.delete(p);
  largestOf.clear();
  measure(cur.value, true);
  for (const p of open) measure(p, true);
});

const rowOf = (u: DiskUsage, it: DiskUsage['items'][number], depth: number): Row => ({
  name: it.name, dir: it.dir, at: u.path, path: join(u.path, it.name), size: it.size, files: it.files, dirs: it.dirs,
  time: Date.parse(it.mtime) || 0, depth, whole: u.size, canOpen: it.dir && it.files + it.dirs > 0,
});
const order = (a: Row, b: Row) => {
  if (sort.by === 'name') return a.name.localeCompare(b.name, 'de', { numeric: true }) * -sort.dir;
  const key = sort.by === 'share' ? 'size' : sort.by;
  return (a[key] - b[key]) * sort.dir || a.name.localeCompare(b.name, 'de', { numeric: true });
};

function treeRows(): Row[] {
  const out: Row[] = [];
  const add = (path: string, depth: number) => {
    const u = usage.get(path);
    if (!u) return;
    for (const r of u.items.map((it) => rowOf(u, it, depth)).sort(order)) {
      out.push(r);
      if (r.canOpen && open.has(r.path)) add(r.path, depth + 1);
    }
  };
  add(cur.value, 0);
  return out;
}
function largestRows(): Row[] {
  const u = here.value;
  if (!u) return [];
  const list = kind.value ? largestKind(cur.value, kind.value.kind) ?? [] : u.largest;
  return list.map((f) => ({ name: f.name, dir: false, at: f.at, path: join(f.at, f.name), size: f.size, files: 1, dirs: 0, time: Date.parse(f.mtime) || 0, depth: 0, whole: u.size, canOpen: false }));
}
function mapRows(): Row[] {
  const u = here.value;
  return u ? u.items.map((it) => rowOf(u, it, 0)).filter((r) => r.size > 0).sort((a, b) => b.size - a.size) : [];
}
const rows = computed<Row[]>(() => (mode.value === 'tree' ? treeRows() : mode.value === 'largest' ? largestRows() : mode.value === 'map' ? mapRows() : []));
const types = computed(() => {
  const list = [...(here.value?.types ?? [])].sort((a, b) => b.size - a.size);
  return { list, whole: list.reduce((a, x) => a + x.size, 0) };
});
const kindName = (k: string, server: string) => { const own = t(`diskusage.kinds.${k}`); return own.startsWith('diskusage.') ? server : own; };

const blocks = computed(() => {
  const list = rows.value;
  if (mode.value !== 'map' || !list.length || !box.w) return [];
  const max = list[0]!.size;
  return squarify(list.map((r, i) => ({ r, i, v: r.size })), 0, 0, box.w, box.h).map((b) => {
    const { r, i } = b.it, pad = 2, x = b.x + pad, y = b.y + pad, bw = Math.max(0, b.w - pad * 2), bh = Math.max(0, b.h - pad * 2);
    const mix = r.dir ? Math.round(18 + (r.size / max) * 37) : 0;
    return {
      r, i, x, y, bw, bh,
      fill: r.dir ? `color-mix(in srgb, var(--accent) ${mix}%, var(--carbon-surface3))` : 'var(--carbon-surface3)',
      ink: r.dir && mix > 45 ? 'var(--accent-contrast)' : 'var(--carbon-text)',
      name: fit(r.name, bw - 14), size: fit(fmtSize(r.size), bw - 14),
    };
  });
});

const summary = computed(() => {
  if (sel.size) return t('diskusage.picked', { n: num(sel.size), size: fmtSize([...sel.values()].reduce((a, r) => a + r.size, 0)) || '0 B' });
  const u = here.value;
  if (!u) return ' ';
  return t('diskusage.summary', { size: fmtSize(u.size) || '0 B', files: plural(u.files, 'ui.files'), dirs: plural(u.dirs, 'ui.folders') });
});
watch([summary, () => sel.size], () => {
  setFoot(props.win, summary.value, [
    { label: t('diskusage.delete'), g: 'IconTrash', disabled: !sel.size, run: () => { remove(); return false; } },
    { label: t('diskusage.openHere'), g: 'IconFolderOpen', primary: true, run: () => { void F.go(cur.value); } },
  ]);
}, { immediate: true });

function show(p: string) {
  cur.value = p;
  open.clear();
  sel.clear();
  focus.value = null;
  kind.value = null;
  body.value?.focus();
}
const steps = folderSteps(props.win, () => cur.value, show);
const go = (p: string) => steps.go(p);
function setView(v: View) {
  mode.value = v;
  body.value?.focus();
}

function pick(r: Row | undefined, e: { shiftKey?: boolean; ctrlKey?: boolean; metaKey?: boolean } = {}) {
  if (!r) return;
  const list = rows.value;
  if (e.shiftKey && anchor) {
    const a = list.findIndex((x) => x.path === anchor), b = list.indexOf(r);
    if (!(e.ctrlKey || e.metaKey)) sel.clear();
    for (let i = Math.min(a, b); i <= Math.max(a, b); i++) if (i >= 0) sel.set(list[i]!.path, list[i]!);
  } else if (e.ctrlKey || e.metaKey) {
    if (sel.has(r.path)) sel.delete(r.path); else sel.set(r.path, r);
    anchor = r.path;
  } else {
    sel.clear();
    sel.set(r.path, r);
    anchor = r.path;
  }
  focus.value = r.path;
  nextTick(() => body.value?.querySelector('.du-row.cur')?.scrollIntoView({ block: 'nearest' }));
}
function toggle(r: Row, state = !open.has(r.path)) {
  if (!r.canOpen) return;
  if (state) { open.add(r.path); measure(r.path); } else open.delete(r.path);
  focus.value = r.path;
}

function remove() {
  const picked = [...sel.values()];
  if (!picked.length) return toast(t('diskusage.pickFirst'), 'warn');
  if (inArchive(cur.value)) return toast(t('ui.archiveOnly'), 'warn');
  const items = outermost(picked).map((r) => toEntry({ name: r.name, dir: r.dir, size: r.size, time: r.time, mode: '', owner: '', group: '', at: r.at }));
  deleteItems(items, cur.value, { after: () => { sel.clear(); focus.value = null; } });
}
async function reveal(r: Row) {
  props.close();
  if (view.id !== 'files') goView('files');
  await F.go(r.at);
  const i = F.entries().findIndex((k) => k.name === r.name);
  if (i >= 0) F.T().cursor = i;
}

const rowAt = (el: Element) => rows.value[Number(el.closest<HTMLElement>('[data-dui]')!.dataset.dui)];
function click(e: MouseEvent) {
  const target = e.target as Element;
  const s = target.closest<HTMLElement>('[data-du-sort]');
  if (s) {
    const by = s.dataset.duSort as SortKey;
    Object.assign(sort, { by, dir: sort.by === by ? -sort.dir : by === 'name' ? 1 : -1 });
    return;
  }
  const k = target.closest<HTMLElement>('[data-du-kind]');
  if (k) {
    kind.value = { kind: k.dataset.duKind!, name: k.dataset.duName!, glyph: k.dataset.duGlyph! };
    sel.clear();
    return setView('largest');
  }
  if (target.closest('[data-du-allkinds]')) { kind.value = null; return; }
  const row = target.closest('[data-dui]');
  if (!row) return;
  const r = rowAt(row)!;
  if (target.closest('[data-du-tog]')) return toggle(r);
  if (mode.value === 'map' && r.dir && !(e.ctrlKey || e.metaKey || e.shiftKey)) return go(r.path);
  pick(r, e);
}
function dblclick(e: MouseEvent) {
  const row = (e.target as Element).closest('[data-dui]');
  if (!row || mode.value === 'map') return;
  const r = rowAt(row)!;
  if (r.dir) go(r.path); else void reveal(r);
}
function context(e: MouseEvent) {
  const row = (e.target as Element).closest('[data-dui]');
  if (!row) return;
  e.preventDefault();
  e.stopPropagation();
  const r = rowAt(row)!;
  if (!sel.has(r.path)) pick(r);
  const many = sel.size > 1;
  menu(e.clientX, e.clientY, [
    ...(r.canOpen && mode.value === 'tree' ? [{ label: open.has(r.path) ? t('diskusage.collapse') : t('diskusage.expand'), g: open.has(r.path) ? 'IconCollapse' : 'IconExpand', run: () => toggle(r) }] : []),
    ...(r.dir ? [{ label: t('diskusage.fromHere'), g: 'IconDashboard', run: () => go(r.path) }] : []),
    { label: t('diskusage.showInPanel'), g: 'IconFolderOpen', run: () => reveal(r) },
    { label: t('diskusage.copyPath'), g: 'IconCopy', run: () => copyText(r.path) },
    '-',
    { label: many ? t('diskusage.deleteMany', { n: sel.size }) : t('diskusage.delete'), g: 'IconTrash', run: remove },
  ]);
}
function key(e: KeyboardEvent) {
  if (e.key === 'Backspace' && cur.value !== '/') { e.preventDefault(); return go(parent(cur.value)); }
  if (e.key === 'Delete') { e.preventDefault(); return remove(); }
  const list = rows.value;
  if (!list.length || mode.value === 'map' || mode.value === 'types') return;
  const i = list.findIndex((r) => r.path === focus.value), r = list[i];
  const step = (to: number) => {
    e.preventDefault();
    const n = list[Math.max(0, Math.min(list.length - 1, to))];
    pick(n, e.shiftKey ? { shiftKey: true } : {});
  };
  if (e.key === 'ArrowDown') return step(i + 1);
  if (e.key === 'ArrowUp') return step(i < 0 ? 0 : i - 1);
  if (e.key === 'Home') return step(0);
  if (e.key === 'End') return step(list.length - 1);
  if (!r) return;
  if (e.key === 'ArrowRight' && mode.value === 'tree') {
    e.preventDefault();
    if (r.canOpen && !open.has(r.path)) toggle(r, true);
    else if ((list[i + 1]?.depth ?? -1) > r.depth) pick(list[i + 1]);
  }
  if (e.key === 'ArrowLeft' && mode.value === 'tree') {
    e.preventDefault();
    if (open.has(r.path)) toggle(r, false);
    else pick(list.find((x) => x.path === r.at));
  }
  if (e.key === 'Enter') {
    e.preventDefault();
    if (mode.value === 'tree' && r.canOpen) toggle(r);
    else if (r.dir) go(r.path);
    else void reveal(r);
  }
  if (e.key === 'a' && (e.ctrlKey || e.metaKey)) { e.preventDefault(); list.forEach((x) => sel.set(x.path, x)); }
}

let ro: ResizeObserver | null = null;
onMounted(() => {
  const measureBox = () => { const r = body.value!.getBoundingClientRect(); Object.assign(box, { w: r.width, h: r.height }); };
  measureBox();
  ro = new ResizeObserver(measureBox);
  ro.observe(body.value!);
  setTimeout(() => body.value?.focus(), 0);
});
onBeforeUnmount(() => { ro?.disconnect(); steps.close(); });

const rowClass = (r: Row) => ['du-row', { sel: sel.has(r.path), cur: focus.value === r.path, dir: r.dir }];
const share = (part: number, whole: number) => ({ n: Math.round(pct(part, whole)) });
const rel = (r: Row) => (r.at === cur.value ? '' : r.at.slice(cur.value === '/' ? 1 : cur.value.length + 1));
const bodyClass = computed(() => `du-body du-${mode.value}`);
const role = computed(() => ({ tree: 'tree', largest: 'listbox', types: 'list', map: 'img' })[mode.value]);
</script>

<template>
  <div class="du-top"><div id="du-crumbs" class="pathbar du-pathbar"><Crumbs :path="cur" attr="data-du-cd" @go="go" /></div><Selector id="du-view" :model-value="mode" :items="segItems(VIEWS)" size="sm" activation="manual" inline button-height pairs-on-phone @press="setView" /></div>
  <SubCard class="du-well">
    <div id="du-body" ref="body" :class="bodyClass" tabindex="0" :role="role" @click="click" @dblclick="dblclick" @contextmenu="context" @keydown="key">
      <template v-if="mode === 'tree'">
        <div class="du-row du-head" role="row"><button v-for="id in COLS" :key="id" class="du-sort" :class="{ on: sort.by === id }" :data-du-sort="id">{{ t(`diskusage.cols.${id}`) }}<G v-if="sort.by === id" :n="sort.dir > 0 ? 'IconMoveUp' : 'IconMoveDown'" /></button></div>
        <template v-if="rows.length">
          <div v-for="(r, i) in rows" :key="r.path" :class="rowClass(r)" :data-dui="i" role="treeitem" :aria-level="r.depth + 1" :aria-selected="sel.has(r.path)" :aria-expanded="r.canOpen ? open.has(r.path) : undefined">
            <span class="du-name" :style="{ '--d': r.depth }"><button v-if="r.canOpen" class="tree-tog" :class="{ open: open.has(r.path) }" data-du-tog tabindex="-1" :aria-label="open.has(r.path) ? t('diskusage.collapse') : t('diskusage.expand')"><G n="IconExpand" /></button><span v-else class="tree-tog"></span><G :n="glyphOf(r)" /><span class="n">{{ r.name }}</span></span>
            <span class="glim-num r">{{ fmtSize(r.size) || '0 B' }}</span><span class="du-share"><Meter quiet :threshold="2" :value="share(r.size, r.whole).n / 100" :label="r.name" /><span class="glim-num">{{ share(r.size, r.whole).n }} %</span></span>
            <span class="glim-num r">{{ r.dir ? num(r.files) : '' }}</span><span class="glim-num r">{{ r.dir ? num(r.dirs) : '' }}</span>
            <span class="glim-num r muted">{{ r.time ? fmtDate(r.time) : '' }}</span>
          </div>
        </template>
        <EmptyState v-else-if="here" nested :glyph="glyphComponent('IconFolder')" :title="t('diskusage.emptyFolder')" />
      </template>
      <template v-else-if="mode === 'largest'">
        <div v-if="kind" class="du-kind"><G :n="kind.glyph" /><span>{{ t('diskusage.only', { name: kind.name }) }}</span><IconButton :tip="t('diskusage.allKinds')" :glyph="glyphComponent('IconClose')" data-du-allkinds /></div>
        <template v-if="rows.length">
          <div v-for="(r, i) in rows" :key="r.path" :class="rowClass(r)" :data-dui="i" role="option" :aria-selected="sel.has(r.path)">
            <span class="du-name"><G :n="glyphOf(r)" /><span class="n">{{ r.name }}</span></span>
            <span class="du-path mono muted"><bdi>{{ rel(r) }}</bdi></span>
            <span class="glim-num r">{{ fmtSize(r.size) || '0 B' }}</span><span class="du-share"><Meter quiet :threshold="2" :value="share(r.size, r.whole).n / 100" :label="r.name" /><span class="glim-num">{{ share(r.size, r.whole).n }} %</span></span>
            <span class="glim-num r muted">{{ r.time ? fmtDate(r.time) : '' }}</span>
          </div>
        </template>
        <EmptyState v-else-if="here" nested :glyph="glyphComponent('IconFile')" :title="t('diskusage.noFiles')" />
      </template>
      <template v-else-if="mode === 'types'">
        <template v-if="types.list.length">
          <button v-for="x in types.list" :key="x.kind" class="du-row du-type" :data-du-kind="x.kind" :data-du-name="kindName(x.kind, x.name)" :data-du-glyph="KIND_GLYPH[x.kind] || 'IconFile'" :data-tip="t('diskusage.kindTip')">
            <span class="du-name"><G :n="KIND_GLYPH[x.kind] || 'IconFile'" /><span class="n">{{ kindName(x.kind, x.name) }}</span></span>
            <span class="du-exts muted">{{ x.exts.slice(0, 5).map((e) => '.' + e).join(' ') }}</span>
            <span class="glim-num r muted">{{ plural(x.files, 'ui.files') }}</span>
            <span class="glim-num r">{{ fmtSize(x.size) || '0 B' }}</span><span class="du-share"><Meter quiet :threshold="2" :value="share(x.size, types.whole).n / 100" :label="x.name" /><span class="glim-num">{{ share(x.size, types.whole).n }} %</span></span>
          </button>
        </template>
        <EmptyState v-else-if="here" nested :glyph="glyphComponent('IconFile')" :title="t('diskusage.noFiles')" />
      </template>
      <template v-else>
        <svg v-if="blocks.length" :width="box.w" :height="box.h" :viewBox="`0 0 ${box.w} ${box.h}`" :aria-label="t('diskusage.mapLabel', { path: cur })">
          <g v-for="b in blocks" :key="b.r.path" class="bk-blk" :class="{ dir: b.r.dir, sel: sel.has(b.r.path) }" :data-dui="b.i" :data-tip="`${b.r.name}, ${fmtSize(b.r.size)}`" :style="{ '--blk': b.fill, '--ink': b.ink }">
            <rect :x="b.x" :y="b.y" :width="b.bw" :height="b.bh" rx="6"></rect>
            <text v-if="b.bw > 54 && b.bh > 24 && b.name" :x="b.x + 8" :y="b.y + 17" class="n">{{ b.name }}</text>
            <text v-if="b.bw > 54 && b.bh > 42 && b.size" :x="b.x + 8" :y="b.y + 33" class="s">{{ b.size }}</text>
          </g>
        </svg>
        <EmptyState v-else-if="here" nested :glyph="glyphComponent('IconFolder')" :title="t('diskusage.nothingBig')" />
      </template>
    </div>
  </SubCard>
</template>

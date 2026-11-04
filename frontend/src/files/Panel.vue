<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue';
import { run } from '@/core/actions';
import { extend } from '@/core/extend';
import { fmtSize } from '@/core/format';
import { G, glyphOf } from '@/core/glyphs';
import { menu } from '@/core/menu';
import { prefs } from '@/core/prefs';
import { Render } from '@/core/Render';
import Cbox from '@/core/Cbox.vue';
import { t } from '@/i18n';
import { useListing } from '@/stores/listing';
import { type Tab, usePanels } from '@/stores/panels';
import { COL, Columns } from './columns';
import type { Entry } from './entries';
import { columnMenu, listMenu, rowMenu, tabMenu } from './menus';
import { inArchive, join, split } from './paths';
import { viewGlyph, viewOf } from './views';

const props = defineProps<{ i: number }>();
const F = usePanels();
const listing = useListing();

const P = computed(() => F.P(props.i));
const tab = computed(() => F.T(props.i));
const list = computed(() => F.entries(props.i));
const parts = computed(() => split(tab.value.path));
const v = computed(() => viewOf(tab.value));
const visible = computed(() => Columns.visible());
const active = computed(() => props.i === F.active);
const vol = computed(() => F.volume(props.i));
const subFolders = computed(() => listing.trees.get(tab.value.path));
const hasDirs = computed(() => listing.lists.get(tab.value.path)?.items.some((k) => k.dir) ?? false);

watch(() => tab.value.path, (p) => { listing.tree(p).catch(() => {}); }, { immediate: true });
watch(list, (l) => { if (tab.value.cursor >= l.length) tab.value.cursor = Math.max(0, l.length - 1); });

const tabName = (tb: Tab) => (listing.placeLabel(tb.path) || split(tb.path).pop() || t('files.server')).replace('::', '');
const volumeOf = (k: Entry) => (k.dir ? subFolders.value?.find((x) => x.name === k.name && x.total) : undefined);
const shownName = (k: Entry) => (prefs.showExt || k.dir || !k.name.includes('.', 1) ? k.name : k.name.slice(0, k.name.lastIndexOf('.')));
const used = (free: number, total: number) => `${Math.round((1 - free / total) * 100)}%`;

function activate() {
  if (!active.value && !prefs.quickview) F.active = props.i;
}

function closeTab(k: number) {
  const p = P.value;
  p.tabs.splice(k, 1);
  p.cur = Math.min(p.cur, p.tabs.length - 1);
}
function middle(e: MouseEvent, k: number) {
  if (e.button !== 1 || P.value.tabs.length < 2) return;
  e.preventDefault();
  closeTab(k);
}
function onTabMenu(e: MouseEvent, k: number) {
  P.value.cur = k;
  tabMenu(e.clientX, e.clientY);
}

// A separator in the path opens the folders below the part on its left, as Explorer's arrows do.
async function psub(e: MouseEvent, under: string) {
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect(), next = tab.value.path;
  const dirs = (await listing.tree(under).catch(() => []))
    .filter((k) => prefs.showHidden || !k.name.startsWith('.'))
    .sort((a, b) => a.name.localeCompare(b.name, 'de', { numeric: true }));
  menu(r.left, r.bottom + 4, dirs.map((k) => {
    const onPath = next === k.path || next.startsWith(k.path + '/');
    return { label: k.label || k.name, g: onPath ? 'IconFolderOpen' : 'IconFolder', run: () => F.go(k.path) };
  }));
}
const under = (k: number) => '/' + parts.value.slice(0, k).join('/');

function onPathbar(e: MouseEvent) {
  if (!(e.target as Element).closest('.pseg, .psep, .pmore')) run('gotoPath');
}
async function pathMore() {
  tab.value.pathOpen = !tab.value.pathOpen;
  await nextTick();
  document.querySelectorAll('.pathbar.open').forEach((p) => { p.scrollLeft = p.scrollWidth; });
}

const pathInput = ref<HTMLInputElement | null>(null);
watch(() => tab.value.editPath, async (on) => {
  if (!on) return;
  await nextTick();
  pathInput.value?.focus();
  pathInput.value?.select();
});
function pathKey(e: KeyboardEvent) {
  if (e.key === 'Enter') F.go((e.target as HTMLInputElement).value.trim() || '/', props.i);
  if (e.key === 'Escape') tab.value.editPath = false;
}
const pathBlur = () => { tab.value.editPath = false; };

function viewMenu(e: MouseEvent) {
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
  menu(r.right - 230, r.bottom + 6, ['details', 'compact', 'list', 'content', 'columns', 'tiles', 'large', 'gallery', 'drives'].map((id) => 'view-' + id));
}

function sortBy(key: string) {
  const tb = tab.value;
  if (tb.sort === key) tb.dir *= -1;
  else Object.assign(tb, { sort: key, dir: 1 });
}

// Two clicks are paired here instead of by dblclick, so a Shift or Ctrl click never opens anything.
let lastClick: { idx?: number; panel?: number; at?: number } = {};
function onListClick(e: MouseEvent) {
  const el = e.target as Element;
  const row = el.closest<HTMLElement>('.row');
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
  } else if (!el.closest('.lhead') && tb.marked.size) {
    // A click on empty space in the list drops the selection, as in Explorer.
    tb.marked = new Set();
  }
}

function onListMenu(e: MouseEvent) {
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

const marked = computed(() => list.value.filter((k) => !k.up && tab.value.marked.has(k.name)));
const count = computed(() => {
  const n = list.value.filter((k) => !k.up).length, tb = tab.value;
  return tb.filter || tb.smart ? t('files.hits', { n }) : tb.flat ? t('files.flatCount', { n }) : n === 1 ? t('files.one') : t('files.many', { n });
});
</script>

<template>
  <section class="panel glim-card" :class="{ active }" :data-panel="i" @click.capture="activate">
    <div class="ptabs" @dblclick.self="run('newTab')">
      <button v-for="(tb, k) in P.tabs" :key="k" class="ptab" :class="{ cur: k === P.cur }" :data-tab="k" :data-tip="tb.path + (tb.locked ? t('files.lockedSuffix') : '')"
        @click="P.cur = k" @contextmenu.prevent="onTabMenu($event, k)" @mousedown.middle.prevent @auxclick="middle($event, k)">
        <G v-if="tb.locked" n="IconPin" />{{ tabName(tb) }}<span v-if="P.tabs.length > 1" class="x" :data-close="k" :data-tip="t('files.closeTab')" @click.stop="closeTab(k)"><G n="IconClose" /></span>
      </button>
      <button class="ptab add" data-newtab :data-tip="t('files.newTab')" @click="run('newTab')"><G n="IconAdd" /></button>
    </div>
    <div class="crumbs">
      <button class="btn icon" data-run="back" :data-tip="t('files.backTip')" @click="run('back')"><G n="IconBack" /></button>
      <input v-if="tab.editPath" :id="`pathedit-${i}`" ref="pathInput" class="pathbar pathedit mono" :value="tab.path" :aria-label="t('files.path')" @keydown="pathKey" @blur="pathBlur">
      <div v-else class="pathbar" :class="{ open: tab.pathOpen }" data-pathbar :data-tip="t('files.pathTip')" @click="onPathbar">
        <button v-if="parts.length > 1" class="pmore" data-pathmore :aria-label="t('files.pathMore')" @click="pathMore">…</button>
        <button class="pseg" data-go="/" @click="F.go('/', i)">{{ t('files.server') }}</button>
        <template v-for="(part, k) in parts" :key="k">
          <button class="psep" :data-psub="under(k)" :aria-label="t('files.foldersIn', { name: split(under(k)).pop() || t('files.server') })" @click="psub($event, under(k))">›</button>
          <button class="pseg" :class="{ last: k === parts.length - 1 }" :data-go="under(k + 1)" @click="F.go(under(k + 1), i)">{{ part.replace('::', '') }}</button>
        </template>
        <button v-if="hasDirs" class="psep" :data-psub="tab.path" :aria-label="t('files.foldersIn', { name: parts.at(-1) || t('files.server') })" @click="psub($event, tab.path)">›</button>
      </div>
      <span class="tools">
        <button class="btn icon" data-run="quickFilter" :data-tip="t('files.searchTip')" @click="run('quickFilter')"><G n="IconSearch" /></button>
        <button class="btn icon" data-labelsmenu :data-tip="t('files.labelsTip')" @click="run('labelsMenu')"><G n="IconHash" /></button>
        <button class="btn icon" data-viewmenu :data-tip="t('files.viewTip', { name: t(`views.${v}`) })" @click="viewMenu"><G :n="viewGlyph(v)" /></button>
        <button v-if="prefs.dual" class="btn icon" data-run="equalPanels" :data-tip="t('files.equalTip')" @click="run('equalPanels')"><G n="IconSync" /></button>
      </span>
    </div>
    <div class="list" :class="`view-${v}`" tabindex="0" :style="{ '--cols': Columns.template(), '--minw': Columns.minWidth() }" @click="onListClick" @contextmenu="onListMenu">
      <div class="lhead" data-lhead>
        <button v-for="id in visible" :key="id" class="lh" :class="{ r: COL[id]!.r, sorted: tab.sort === Columns.sortKey(COL[id]!) }" :data-sort="Columns.sortKey(COL[id]!)" :data-colid="id" @click="sortBy(Columns.sortKey(COL[id]!))">{{ COL[id]!.label() }}<G v-if="tab.sort === Columns.sortKey(COL[id]!)" :n="tab.dir > 0 ? 'IconMoveUp' : 'IconMoveDown'" /><span v-if="!COL[id]!.fixed" class="lh-grip" :data-grip="id" aria-hidden="true"></span></button>
      </div>
      <div v-for="(k, idx) in list" :key="(k.at || '') + k.name" class="row"
        :class="{ dir: k.dir, hidden: k.hidden || k.name.startsWith('.'), marked: tab.marked.has(k.name), cursor: idx === tab.cursor }" :data-idx="idx" draggable="true">
        <template v-for="id in visible" :key="id">
          <span v-if="id === 'name'" class="name">
            <span class="glyph"><Cbox v-if="tab.marked.has(k.name)" on /><G v-else :n="glyphOf(k)" /></span><span class="t">{{ shownName(k) }}</span>
            <span v-if="k.at && k.at !== tab.path" class="link">{{ t('files.foundIn', { path: k.at.slice(tab.path.length + 1) }) }}</span>
            <span v-if="k.link" class="link">→ {{ k.link }}</span>
            <Render v-for="(badge, n) in extend.nameBadges" :key="n" :node="badge(k, tab.path)" />
          </span>
          <span v-else-if="id === 'size' && volumeOf(k)" class="cell r glim-num vcell"><span class="vmeter" :class="{ full: 1 - volumeOf(k)!.free! / volumeOf(k)!.total! > 0.9 }" :title="t('files.freeOf', { free: fmtSize(volumeOf(k)!.free), total: fmtSize(volumeOf(k)!.total) })"><i :style="{ width: used(volumeOf(k)!.free!, volumeOf(k)!.total!) }"></i></span>{{ t('files.free', { size: fmtSize(volumeOf(k)!.free) }) }}</span>
          <span v-else class="cell glim-num" :class="{ r: COL[id]!.r, mono: COL[id]!.mono }">{{ COL[id]!.value(k, tab.path) }}</span>
        </template>
      </div>
      <div v-if="!list.length && F.loaded(i)" class="empty">
        <G :n="tab.filter ? 'IconSearch' : 'IconFolder'" />
        <div>{{ tab.filter ? t('files.nothingFound', { q: tab.filter }) : t('files.empty') }}</div>
        <div class="muted" style="font-size:12px">{{ tab.filter ? (tab.deep === false ? t('files.searchedHere') : t('files.searchedDeep')) : t('files.emptyHint') }}</div>
      </div>
    </div>
    <div class="lfoot glim-num">
      <span v-if="marked.length">{{ t('files.marked', { n: marked.length, size: fmtSize(marked.reduce((a, k) => a + k.size, 0)) }) }}<button class="lclear" data-run="unselect" :data-tip="t('files.unselectTip')" :aria-label="t('files.unselect')" @click="run('unselect')"><G n="IconClose" /></button></span>
      <span v-else>{{ count }}</span>
      <span v-if="prefs.dual" class="pdots" aria-hidden="true"><i v-for="n in [0, 1]" :key="n" :class="{ on: n === i }" :data-pswitch="n" @click="F.active = n"></i></span>
      <Render v-for="(slot, n) in extend.footerSlots" :key="n" :node="slot(i)" />
      <span class="grow"></span>
      <span v-if="inArchive(tab.path)">{{ t('files.inArchive') }}</span>
      <button v-else-if="vol" class="lfree" :data-du="vol.path" :data-tip="t('files.diskUsage', { name: vol.label || vol.path })" @click="run('diskUsage', vol.path)">
        <span>{{ t('files.free', { size: fmtSize(vol.free) }) }}</span><span class="meter"><i :style="{ width: used(vol.free, vol.total) }"></i></span>
      </button>
    </div>
  </section>
</template>

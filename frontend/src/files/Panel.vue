<script setup lang="ts">
// One panel: its tabs on the top edge, the path row with its tools, the list and the footer.
import { computed, nextTick, ref, watch } from 'vue';
import { actions, run } from '@/core/actions';
import { extend } from '@/core/extend';
import { fmtSize } from '@/core/format';
import { G } from '@/core/glyphs';
import { menu } from '@/core/menu';
import { prefs } from '@/core/prefs';
import { Render } from '@/core/Render';
import { t } from '@/i18n';
import { useListing } from '@/stores/listing';
import { type Tab, usePanels } from '@/stores/panels';
import FileList from './FileList.vue';
import { jump } from './keys';
import { tabMenu } from './menus';
import PathBar from './PathBar.vue';
import { inArchive, split } from './paths';
import { VIEWS, viewGlyph } from './views';

const props = defineProps<{ i: number }>();
const F = usePanels();
const listing = useListing();

const P = computed(() => F.P(props.i));
const tab = computed(() => F.T(props.i));
const list = computed(() => F.entries(props.i));
const v = computed(() => F.viewOf(tab.value));
const active = computed(() => props.i === F.active);
const vol = computed(() => F.volume(props.i));

const tabName = (tb: Tab) => (listing.placeLabel(tb.path) || split(tb.path).pop() || t('files.server')).replace('::', '');
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
  F.active = props.i;
  P.value.cur = k;
  tabMenu(e.clientX, e.clientY);
}
function viewMenu(e: MouseEvent) {
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
  menu(r.right - 230, r.bottom + 6, VIEWS.map(([id]) => 'view-' + id));
}

// The filter field keeps the focus while the list below follows each letter.
const qf = ref<HTMLInputElement | null>(null);
watch(() => tab.value.qf && active.value, async (open) => {
  if (!open) return;
  await nextTick();
  qf.value?.focus();
  qf.value?.setSelectionRange(tab.value.filter.length, tab.value.filter.length);
}, { immediate: true });
function onFilter(e: Event) {
  Object.assign(tab.value, { filter: (e.target as HTMLInputElement).value, cursor: 0 });
}
function filterKey(e: KeyboardEvent) {
  if (e.key === 'Escape') { e.preventDefault(); closeFilter(); }
  if (e.key === 'Enter' || e.key === 'ArrowDown') { e.preventDefault(); document.querySelector<HTMLElement>('.panel.active .list')?.focus(); }
}
const closeFilter = () => Object.assign(tab.value, { qf: false, filter: '', cursor: 0 });
function toggleDeep() {
  Object.assign(tab.value, { deep: tab.value.deep === false, cursor: 0 });
  qf.value?.focus();
}

const marked = computed(() => list.value.filter((k) => !k.up && tab.value.marked.has(k.name)));
const count = computed(() => {
  const n = list.value.filter((k) => !k.up).length, tb = tab.value;
  return tb.filter || tb.smart ? t('files.hits', { n }) : tb.flat ? t('files.flatCount', { n }) : n === 1 ? t('files.one') : t('files.many', { n });
});
</script>

<template>
  <section class="panel glim-card" :class="{ active }" :data-panel="i" @click.capture="activate">
    <div class="ptabs" @dblclick.self="F.active = i; run('newTab')">
      <button v-for="(tb, k) in P.tabs" :key="k" class="ptab" :class="{ cur: k === P.cur }" :data-tab="k" :data-tip="tb.path + (tb.locked ? t('files.lockedSuffix') : '')"
        @click="P.cur = k" @contextmenu.prevent="onTabMenu($event, k)" @mousedown.middle.prevent @auxclick="middle($event, k)">
        <G v-if="tb.locked" n="IconPin" />{{ tabName(tb) }}<span v-if="P.tabs.length > 1" class="x" :data-close="k" :data-tip="t('files.closeTab')" @click.stop="closeTab(k)"><G n="IconClose" /></span>
      </button>
      <button class="ptab add" data-newtab :data-tip="t('files.newTab')" @click="F.active = i; run('newTab')"><G n="IconAdd" /></button>
    </div>
    <div class="crumbs">
      <button class="btn icon" data-run="back" :data-tip="t('files.backTip')" @click="F.active = i; run('back')"><G n="IconBack" /></button>
      <PathBar :i="i" />
      <span class="tools">
        <div v-if="tab.qf" class="qfilter">
          <G n="IconSearch" /><input :id="`qf-${i}`" ref="qf" :value="tab.filter" :placeholder="t('files.filterPlaceholder')" :aria-label="t('files.filterLabel')" spellcheck="false" autocomplete="off" @input="onFilter" @keydown="filterKey">
          <button v-if="tab.filter && actions.saveSearch" class="btn icon" data-qfsave :data-tip="t('files.filterSave')" @click="run('saveSearch')"><G n="IconSave" /></button>
          <button class="qdeep" :class="{ on: tab.deep !== false }" :aria-pressed="tab.deep !== false" :aria-label="t('files.filterDeepLabel')" data-qfdeep
            :data-tip="tab.deep === false ? t('files.filterHere') : t('files.filterDeep')" @click="toggleDeep"><G n="IconFolderOpen" /></button>
          <button class="btn icon" data-qfclose :data-tip="t('files.filterClose')" @click="closeFilter"><G n="IconClose" /></button>
        </div>
        <button v-else class="btn icon" data-run="quickFilter" :data-tip="t('files.searchTip')" @click="F.active = i; run('quickFilter')"><G n="IconSearch" /></button>
        <button class="btn icon" data-labelsmenu :data-tip="t('files.labelsTip')" @click="run('labelsMenu')"><G n="IconHash" /></button>
        <button class="btn icon" data-viewmenu :data-tip="t('files.viewTip', { name: t(`views.${v}`) })" @click="F.active = i; viewMenu($event)"><G :n="viewGlyph(v)" /></button>
        <button v-if="prefs.dual" class="btn icon" data-run="equalPanels" :data-tip="t('files.equalTip')" @click="F.active = i; run('equalPanels')"><G n="IconSync" /></button>
      </span>
    </div>
    <FileList :key="tab.path + v" :i="i" />
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
    <div v-if="active && jump.text" class="tjump glim-card" :class="{ miss: !jump.found }" role="status">
      <G n="IconSearch" /><span class="mono">{{ jump.text }}</span><small>{{ jump.found ? t('files.jumpNext') : t('files.jumpMiss') }}</small>
    </div>
  </section>
</template>

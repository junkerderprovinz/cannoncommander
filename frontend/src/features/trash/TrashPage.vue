<script setup lang="ts">
// Papierkorb: a full page listing what was deleted, with restore, delete for good and empty.
import { Button, Card, EmptyState, FieldRow, GlyphTile, IconButton, Pill } from '@glimstone/vue';
import { computed, onBeforeUnmount, onMounted } from 'vue';
import type { TrashItem } from '@/api/ops';
import { DAY, fmtDate, fmtSize } from '@/core/format';
import { G, glyphOf, glyphComponent } from '@/core/glyphs';
import { isMenuOpen, menu } from '@/core/menu';
import { go } from '@/core/views';
import { winStack } from '@/core/windows';
import { t } from '@/i18n';
import { useTrash } from '@/stores/trash';
import { objects } from '@/features/jobs/label';
import { purge, restore } from './trash';
import { pick, type SortKey } from './pick';

const trash = useTrash();
const COLS: SortKey[] = ['name', 'from', 'size', 'deleted'];
const at = (e: TrashItem) => Date.parse(e.deletedAt) || 0;
const BY: Record<SortKey, (a: TrashItem, b: TrashItem) => number> = {
  name: (a, b) => a.name.localeCompare(b.name, 'de', { numeric: true }),
  from: (a, b) => a.from.localeCompare(b.from, 'de'),
  size: (a, b) => a.size - b.size,
  deleted: (a, b) => at(a) - at(b),
};
const list = computed(() => trash.items.slice().sort((a, b) => BY[pick.sort](a, b) * pick.dir));
const picked = computed(() => list.value.filter((e) => pick.sel.has(e.id)));
const total = computed(() => list.value.reduce((a, e) => a + e.size, 0));
const next = computed(() => trash.items.reduce<TrashItem | null>((a, e) => (!a || at(e) < at(a) ? e : a), null));
const goneOn = (e: TrashItem) => fmtDate(Date.parse(e.expiresAt) || at(e) + trash.days * DAY);
const clear = () => pick.sel.clear();

function sortBy(k: SortKey) {
  if (pick.sort === k) pick.dir *= -1;
  else Object.assign(pick, { sort: k, dir: k === 'deleted' || k === 'size' ? -1 : 1 });
}
function click(e: TrashItem, i: number, ev: MouseEvent) {
  if (ev.shiftKey && pick.anchor >= 0) {
    const [a, b] = [Math.min(pick.anchor, i), Math.max(pick.anchor, i)];
    for (let k = a; k <= b; k++) pick.sel.add(list.value[k]!.id);
  } else if (ev.ctrlKey || ev.metaKey || (ev.target as Element).closest('[data-mark]') || pick.sel.size) {
    if (pick.sel.has(e.id)) pick.sel.delete(e.id);
    else pick.sel.add(e.id);
    pick.anchor = i;
  } else {
    pick.sel = new Set([e.id]);
    pick.anchor = i;
  }
}
function more(ev: MouseEvent) {
  const r = (ev.currentTarget as HTMLElement).getBoundingClientRect(), sel = picked.value;
  setTimeout(() => menu(r.right - 260, r.bottom + 6, [
    ...(sel.length ? [
      { label: t('trash.restoreSel', { n: sel.length }), g: 'IconRestore', run: () => restore(sel, clear) },
      { label: t('trash.purgeSel', { n: sel.length }), g: 'IconTrash', run: () => purge(sel, false, clear) }, '-',
    ] : []),
    { label: t('trash.selectAll'), g: 'IconSelectAll', key: 'Strg+A', run: () => list.value.forEach((e) => pick.sel.add(e.id)) },
    { label: t('trash.empty'), g: 'IconEmpty', run: () => purge(list.value, true, clear) },
  ]), 0);
}

// The page's own keys come before the app's key map, which would act on the file panels behind it.
function keys(e: KeyboardEvent) {
  if (winStack.length || isMenuOpen() || (e.target as Element).closest?.('input, textarea')) return;
  const combo = (e.ctrlKey || e.metaKey ? 'Ctrl+' : '') + e.key;
  const run: Record<string, () => unknown> = {
    'Ctrl+a': () => list.value.forEach((x) => pick.sel.add(x.id)),
    Delete: () => purge(picked.value, false, clear),
    Enter: () => restore(picked.value, clear),
    ...(pick.sel.size ? { Escape: clear } : {}),
  };
  if (!run[combo]) return;
  e.preventDefault();
  e.stopPropagation();
  run[combo]!();
}
onMounted(() => { trash.load().catch(() => {}); addEventListener('keydown', keys, true); });
onBeforeUnmount(() => removeEventListener('keydown', keys, true));
</script>

<template>
  <div v-if="!list.length" class="page-inner ct">
    <Card :title="t('trash.title')">
      <EmptyState class="ct-empty" nested :glyph="glyphComponent('IconTrash')" :title="t('trash.emptyTitle')" :hint="t('trash.emptyText', { days: trash.days })"><Button :label="t('trash.toFiles')" label-key="IconFiles" data-go-view="files" @click="go('files')" /></EmptyState>
    </Card>
  </div>
  <div v-else class="page-inner ct">
    <Card :title="t('trash.title')">
      <FieldRow :label="t('trash.content')"><Pill class="glim-num">{{ t('trash.contentPill', { what: objects(list.length), size: fmtSize(total) }) }}</Pill></FieldRow>
      <FieldRow v-if="next" :label="t('trash.next')" :hint="t('trash.nextTip', { days: trash.days })"><Pill>{{ next.name }}</Pill><Pill class="glim-num">{{ t('trash.nextOn', { date: goneOn(next) }) }}</Pill></FieldRow>
    </Card>
    <Card class="ct-card" :title="t('trash.listTitle')">
      <div class="ct-head" role="row"><span></span><span></span>
        <button v-for="k in COLS" :key="k" :data-tsort="k" :class="[{ sorted: pick.sort === k, r: k === 'size' || k === 'deleted' }, `col-${k}`]" @click="sortBy(k)">{{ t(`trash.col.${k}`) }}<G v-if="pick.sort === k" :n="pick.dir > 0 ? 'IconMoveUp' : 'IconMoveDown'" /></button>
        <Button id="ct-more" :label="t('trash.more')" label-key="IconMore" @click="more" />
      </div>
      <div class="ct-list" :class="{ picking: picked.length }" role="listbox" aria-multiselectable="true">
        <div v-for="(e, i) in list" :key="e.id" class="ct-row" :class="{ sel: pick.sel.has(e.id) }" :data-i="i" role="option" :aria-selected="pick.sel.has(e.id)" @click="click(e, i, $event)">
          <button class="ct-mark" data-mark :aria-label="t('trash.mark')"><G n="IconCheck" /></button>
          <GlyphTile :glyph="glyphComponent(e.dir ? 'IconFolder' : glyphOf({ name: e.name }))" :active="pick.sel.has(e.id)" />
          <span class="t">{{ e.name }}</span>
          <Pill class="mono col-from" :data-tip="e.from">{{ e.from }}</Pill>
          <Pill class="glim-num col-size">{{ fmtSize(e.size) || '0 B' }}</Pill>
          <Pill class="glim-num col-deleted" :data-tip="t('trash.byTip', { by: e.by, date: goneOn(e) })">{{ fmtDate(at(e)) }}, {{ e.by }}</Pill>
          <span class="ct-acts">
            <IconButton :tip="t('trash.restore')" :glyph="glyphComponent('IconRestore')" data-act="restore" @click.stop="restore([e], clear)" />
            <IconButton :tip="t('trash.purge')" :glyph="glyphComponent('IconTrash')" data-act="purge" @click.stop="purge([e], false, clear)" />
          </span>
        </div>
      </div>
    </Card>
    <div v-if="picked.length" class="ct-fab"><Button id="ct-restore" size="key" :label="picked.length === 1 ? t('trash.restore') : t('trash.restoreMany', { n: picked.length })" label-key="IconRestore" tone="accent" @click="restore(picked, clear)" /></div>
  </div>
</template>

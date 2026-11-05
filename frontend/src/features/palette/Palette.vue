<script setup lang="ts">
// The command search over the list: type to narrow, the arrows or Tab to pick, Enter to run.
import { GlyphField, Kbd, RowMark } from '@glimstone/vue';
import { computed, nextTick, ref, watch } from 'vue';
import { G, glyphComponent } from '@/core/glyphs';
import { t } from '@/i18n';
import { type Line, lines, type Row } from './rows';
import { pal, pick } from './state';

const q = ref('');
const input = ref<InstanceType<typeof GlyphField> | null>(null);
const list = ref<HTMLElement | null>(null);
const rows = computed<Line[]>(() => (pal.open ? lines(q.value) : []));
const pickable = computed(() => rows.value.flatMap((r, i) => ('head' in r ? [] : [i])));
const cur = ref(-1);

watch(rows, () => { cur.value = pickable.value[0] ?? -1; });
watch(() => pal.open, async (open) => {
  if (!open) return;
  q.value = '';
  cur.value = pickable.value[0] ?? -1;
  await nextTick();
  input.value?.input?.focus();
}, { immediate: true });
watch(() => pal.focus, () => input.value?.input?.focus());

// The first row brings the list back to its top, so the heading above it shows again.
watch(cur, async (i) => {
  await nextTick();
  if (i === pickable.value[0]) { if (list.value) list.value.scrollTop = 0; }
  else document.getElementById(`pal-${i}`)?.scrollIntoView({ block: 'nearest' });
});

const step = (d: number) => {
  const p = pickable.value;
  if (p.length) cur.value = p[(p.indexOf(cur.value) + d + p.length) % p.length]!;
};
function onKey(e: KeyboardEvent) {
  const k = e.key === 'Tab' ? (e.shiftKey ? 'ArrowUp' : 'ArrowDown') : e.key;
  if (k === 'ArrowDown' || k === 'ArrowUp') { e.preventDefault(); step(k === 'ArrowDown' ? 1 : -1); }
  else if (k === 'Enter') { e.preventDefault(); pick(rows.value[cur.value]); }
  else if (k === 'Escape' || ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k')) { e.preventDefault(); pal.open = false; }
}
function onDown(e: MouseEvent) {
  if (e.target === e.currentTarget) pal.open = false;
  else if (e.target !== input.value) e.preventDefault();
}
const row = (r: Line) => r as Row;
</script>

<template>
  <div v-if="pal.open" class="pal-scrim glim-modal-backdrop" @mousedown="onDown">
    <div class="pal glim-card" role="dialog" :aria-label="t('palette.title')">
      <GlyphField id="pal-q" ref="input" v-model="q" class="pal-q" :glyph="glyphComponent('IconSearch')" role="combobox" aria-expanded="true" aria-controls="pal-list"
        aria-autocomplete="list" :aria-activedescendant="cur >= 0 ? `pal-${cur}` : ''" :placeholder="t('palette.placeholder')" autocomplete="off" spellcheck="false" @keydown="onKey" />
      <div id="pal-list" ref="list" class="pal-list" role="listbox" :aria-label="t('palette.list')">
        <template v-if="rows.length">
          <template v-for="(r, i) in rows" :key="i">
            <div v-if="'head' in r" class="head" role="presentation">{{ r.head }}</div>
            <div v-else :id="`pal-${i}`" class="pal-row" :class="{ off: !!row(r).why, cur: i === cur }" role="option" :aria-selected="i === cur" :aria-disabled="!!row(r).why"
              :data-i="i" :data-tip="row(r).why || undefined" @click="pick(r)" @mousemove="cur !== i && (cur = i)">
              <RowMark v-if="row(r).a?.checked" :on="row(r).a!.checked!()" /><G v-else-if="row(r).g" :n="row(r).g!" /><span v-else class="ph"></span>
              <span class="t">{{ row(r).name }}</span>
              <span v-if="row(r).group" class="grp">{{ row(r).group }}</span>
              <Kbd v-if="row(r).a?.key">{{ row(r).a!.key }}</Kbd>
            </div>
          </template>
        </template>
        <div v-else class="pal-none">{{ t('palette.none', { q: q.trim() }) }}</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// The finds of Aufräumen by group, each one ticked on its own or the whole group at once.
import { pathField } from '@/ui/picker';
import { FieldRow, InfoBubble, PathField, Pill } from '@glimstone/vue';
import { computed } from 'vue';
import type { CleanupFind, CleanupScan } from '@/api/tools';
import { fmtSize } from '@/core/format';
import { Checkbox } from '@glimstone/vue';
import { G } from '@/core/glyphs';
import { t } from '@/i18n';
import { plural } from '@/ui/kit';
import { GROUPS, keyOf } from './groups';

export interface CleanState { path: string; root: string; scan: CleanupScan | null; missing: boolean; picked: Set<string> }
const props = defineProps<{ st: CleanState }>();

const groups = computed(() => GROUPS.map((gr) => ({ ...gr, rows: props.st.scan?.groups[gr.id] ?? [] })).filter((gr) => gr.rows.length));
const total = computed(() => groups.value.reduce((a, gr) => a + gr.rows.length, 0));
const shown = (f: CleanupFind) => f.path.slice(props.st.root === '/' ? 0 : props.st.root.length) || f.path;
function noteOf(group: string, f: CleanupFind) {
  if (group === 'temp') return fmtSize(f.size) || '0 B';
  if (group === 'emptyFile') return '0 B';
  if (group === 'link') return t('cleanup.notes.pointsTo', { target: f.note || '' });
  if (group === 'ext') return t('cleanup.notes.isA', { kind: t(`cleanup.kinds.${f.note}`) });
  return f.note ? t(`cleanup.notes.${f.note}`) : '';
}
function flip(key: string) {
  const picked = props.st.picked;
  if (picked.has(key)) picked.delete(key); else picked.add(key);
}
function flipGroup(group: string, rows: CleanupFind[]) {
  const keys = rows.map((f) => keyOf(group, f)), every = keys.every((k) => props.st.picked.has(k));
  keys.forEach((k) => (every ? props.st.picked.delete(k) : props.st.picked.add(k)));
}
const all = (group: string, rows: CleanupFind[]) => rows.every((f) => props.st.picked.has(keyOf(group, f)));
</script>

<template>
  <div class="well glim-well bk-form"><FieldRow :label="t('cleanup.searchIn')" :hint="t('cleanup.searchInTip')"><PathField v-model="st.path" v-bind="pathField('cl-in')" /></FieldRow></div>
  <div class="well glim-well bk-fill bk-cleanwell">
    <div class="bk-tools"><span id="cl-sum" class="eyebrow">{{ st.scan && total ? t('cleanup.findsIn', { finds: plural(total, 'cleanup.findWord'), objects: plural(st.scan.scanned, 'cleanup.objectsIn') }) : t('cleanup.finds') }}</span><InfoBubble v-if="st.scan?.capped" :tip="t('cleanup.capped')" /></div>
    <div id="cl-list" class="bk-list">
      <div v-if="st.missing" class="empty"><G n="IconFolder" /><div>{{ t('ui.noFolder') }}</div></div>
      <template v-else-if="st.scan">
        <section v-for="gr in groups" :key="gr.id" class="bk-group">
          <div class="cl-head">
            <Checkbox slim class="cl-check" :model-value="all(gr.id, gr.rows)" :data-cl-group="gr.id" :label="t('cleanup.pickAll', { group: t(`cleanup.groups.${gr.id}.label`) })" @update:model-value="flipGroup(gr.id, gr.rows)" />
            <span class="cl-g"><G :n="gr.g" /></span><span class="bk-name">{{ t(`cleanup.groups.${gr.id}.label`) }}</span><InfoBubble :tip="t(`cleanup.groups.${gr.id}.tip`)" />
            <Pill class="glim-num">{{ gr.rows.length }}</Pill><Pill>{{ gr.act === 'delete' ? t('cleanup.trash') : t('cleanup.rename') }}</Pill>
          </div>
          <div v-for="f in gr.rows" :key="f.path" class="cl-row" :class="{ on: st.picked.has(keyOf(gr.id, f)) }">
            <Checkbox slim class="cl-check" :model-value="st.picked.has(keyOf(gr.id, f))" :data-cl="keyOf(gr.id, f)" :label="f.path" @update:model-value="flip(keyOf(gr.id, f))" />
            <span class="bk-path mono" :title="f.path"><bdi>{{ shown(f) }}</bdi></span>
            <span v-if="f.to" class="cl-to mono"><G n="IconForward" />{{ f.to.split('/').pop() }}</span><Pill>{{ noteOf(gr.id, f) }}</Pill>
          </div>
        </section>
        <div v-if="!groups.length" class="empty"><G n="IconCheck" /><div>{{ t('cleanup.clean') }}</div><div class="muted" style="font-size: 12px">{{ t('cleanup.scanned', { objects: plural(st.scan.scanned, 'ui.objects') }) }}</div></div>
      </template>
    </div>
  </div>
</template>

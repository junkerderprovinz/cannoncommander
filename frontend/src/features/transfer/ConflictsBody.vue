<script setup lang="ts">
// Konflikte: each name that exists at the target, both sides side by side, and an answer per row or for all of them.
import { ref } from 'vue';
import type { Choice, ConflictSide, JobConflict } from '@/api/ops';
import { fmtSize, fmtTime } from '@/core/format';
import { G, glyphOf } from '@/core/glyphs';
import { split } from '@/files/paths';
import { t } from '@/i18n';
import Frow from '@/ui/Frow.vue';
import Seg from '@/ui/Seg.vue';
import { CHOICES, pick } from './rules';

const props = defineProps<{ conflicts: JobConflict[]; dest: string; st: { choice: Record<string, Choice>; all: Choice | null } }>();
const CH = CHOICES();
const ALL = [...CH, ['newer', t('transfer.choice.newer'), 'IconUpdate']] as const;
const all = ref<Choice | null>(null);

function forAll(v: Choice) {
  for (const c of props.conflicts) props.st.choice[c.key] = pick(v, c);
  props.st.all = v;
}
function one(c: JobConflict, v: Choice) {
  props.st.choice[c.key] = v;
  all.value = null;
  props.st.all = null;
}
const nameOf = (c: JobConflict) => split(c.key).pop() || c.key;
const newer = (a: ConflictSide, b: ConflictSide) => Date.parse(a.mtime) > Date.parse(b.mtime) + 60000;
const facts = (n: ConflictSide) => (n.dir ? t('transfer.entries', { n: n.entries ?? 0 }) + ', ' : '') + (fmtSize(n.size) || '0 B');
</script>

<template>
  <div class="well glim-well cf-all"><Frow :label="t('transfer.forAll')" :tip="t('transfer.forAllTip')"><Seg id="cf-all" v-model="all" :options="ALL" @press="forAll" /></Frow></div>
  <div class="well glim-well">
    <div class="eyebrow">{{ conflicts.length === 1 ? t('transfer.existsOne', { dest: split(dest).pop() || t('jobs.server') }) : t('transfer.existsMany', { n: conflicts.length, dest: split(dest).pop() || t('jobs.server') }) }}</div>
    <div class="cf-list">
      <div v-for="(c, i) in conflicts" :key="c.key" class="cf-row" :data-i="i">
        <div class="cf-name"><span class="tile"><G :n="c.source.dir ? 'IconFolder' : glyphOf({ name: nameOf(c) })" /></span><b>{{ c.key }}</b></div>
        <div class="cf-sides">
          <div v-for="[side, other, label] in [[c.source, c.target, t('transfer.sideNew')], [c.target, c.source, t('transfer.sideOld')]] as const" :key="label" class="cf-side">
            <span class="cf-lab">{{ label }}</span><span class="pill glim-num">{{ facts(side) }}</span><span class="pill glim-num">{{ fmtTime(Date.parse(side.mtime)) }}</span><span v-if="newer(side, other)" class="pill state">{{ t('transfer.newer') }}</span>
          </div>
        </div>
        <Seg :model-value="st.choice[c.key] ?? null" :options="CH" :data-row="i" @press="one(c, $event)" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// Konflikte: the name asked about, what is wrong with it, and both sides. The foot's buttons answer it; Für alle
// übernehmen lets one of them answer every name left.
import { Badge, Checkbox, Pill } from '@glimstone/vue';
import { computed } from 'vue';
import type { ConflictSide, JobConflict } from '@/api/ops';
import { fmtSize, fmtTime } from '@/core/format';
import { plural, t } from '@/i18n';

const props = defineProps<{ conflicts: JobConflict[]; dest: string; st: { at: number; all: boolean }; later?: boolean }>();
const c = computed(() => props.conflicts[props.st.at]!);
const many = computed(() => props.conflicts.length > 1);
const more = computed(() => props.later || props.conflicts.length - props.st.at > 1);
const newer = (a: ConflictSide, b: ConflictSide) => Date.parse(a.mtime) > Date.parse(b.mtime) + 60000;
const facts = (n: ConflictSide) => (n.dir ? plural(n.entries ?? 0, 'transfer.entries') + ', ' : '') + (fmtSize(n.size) || '0 B');
</script>

<template>
  <div class="cf-row" :data-i="st.at">
    <div class="cf-name"><b>{{ c.key }}</b><Badge tone="fail" size="small">{{ t('transfer.taken') }}</Badge><span v-if="many" class="cf-step glim-num">{{ t('transfer.step', { at: st.at + 1, n: conflicts.length }) }}</span></div>
    <div class="cf-sides">
      <div v-for="[side, other, label] in [[c.source, c.target, t('transfer.sideNew')], [c.target, c.source, t('transfer.sideOld')]] as const" :key="label" class="cf-side">
        <span class="cf-lab">{{ label }}</span><Pill class="glim-num">{{ facts(side) }}</Pill><Pill class="glim-num">{{ fmtTime(Date.parse(side.mtime)) }}</Pill><Pill v-if="newer(side, other)" tone="state">{{ t('transfer.newer') }}</Pill>
      </div>
    </div>
  </div>
  <Checkbox v-if="more" v-model="st.all" class="cf-all" :label="t('transfer.forAll')" />
</template>

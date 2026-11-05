<script setup lang="ts">
// Symbolische Verknüpfung: what the link points to, where it lies and what it is called.
import { pathField } from '@/ui/picker';
import { FieldRow, FieldRowGroup, PathField, Pill, SubCard, TextField } from '@glimstone/vue';
import { onMounted, ref, watchEffect } from 'vue';
import { join, parent, split } from '@/files/paths';
import { t } from '@/i18n';
import { useListing } from '@/stores/listing';
import FinalPath from '@/ui/FinalPath.vue';
import { pressPrimary } from '@/ui/kit';
import { clean, folderState, namesIn } from '@/ui/tools';

export interface LinkState { target: string; where: string; name: string; ok: boolean; missing: boolean }
const props = defineProps<{ st: LinkState }>();
const listing = useListing();
const okDir = folderState(() => props.st.where);
const name = ref<InstanceType<typeof TextField> | null>(null);

const taken = () => namesIn(clean(props.st.where)).has(props.st.name.trim());
const self = () => join(clean(props.st.where), props.st.name.trim()) === props.st.target.trim();
watchEffect(() => {
  const st = props.st, target = st.target.trim(), n = st.name.trim();
  st.ok = !!okDir.value && !!n && !taken() && !self() && target.startsWith('/') && !n.includes('/');
  const there = listing.lists.get(parent(target));
  st.missing = !!target && !!there && !there.items.some((k) => k.name === split(target).pop());
});
watchEffect(() => { const target = props.st.target.trim(); if (target.startsWith('/')) listing.load(parent(target)).catch(() => {}); });
onMounted(() => setTimeout(() => name.value?.input?.select(), 20));
</script>

<template>
  <SubCard class="bk-form" @keydown.enter="pressPrimary">
    <FieldRowGroup>
      <FieldRow :label="t('props.pointsTo')" :hint="t('props.pointsToTip')"><TextField id="l-target" v-model="st.target" spellcheck="false" autocomplete="off" width="grow" mono /></FieldRow>
      <FieldRow :label="t('props.lies')"><PathField v-model="st.where" v-bind="pathField('l-where')" /></FieldRow>
      <FieldRow :label="t('props.name')"><TextField id="l-name" ref="name" v-model="st.name" spellcheck="false" autocomplete="off" width="grow" /></FieldRow>
    </FieldRowGroup>
    <FieldRow :label="t('props.yields')">
      <span id="l-final" class="bk-finalw">
        <Pill v-if="okDir === false" tone="fail">{{ t('ui.noFolder') }}</Pill>
        <Pill v-else-if="!st.name.trim()" tone="fail">{{ t('ui.noName') }}</Pill>
        <Pill v-else-if="taken()" tone="fail">{{ t('props.takenThere') }}</Pill>
        <Pill v-else-if="self()" tone="fail">{{ t('props.self') }}</Pill>
        <FinalPath v-else :path="join(clean(st.where), st.name.trim())" />
      </span>
    </FieldRow>
  </SubCard>
</template>

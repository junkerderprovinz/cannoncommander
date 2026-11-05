<script setup lang="ts">
// Symbolische Verknüpfung: what the link points to, where it lies and what it is called.
import { onMounted, ref, watchEffect } from 'vue';
import { join, parent, split } from '@/files/paths';
import { t } from '@/i18n';
import { useListing } from '@/stores/listing';
import PathField from '@/ui/PathField.vue';
import FinalPath from '@/ui/FinalPath.vue';
import Frow from '@/ui/Frow.vue';
import { pressPrimary } from '@/ui/kit';
import { clean, folderState, namesIn } from '@/ui/tools';

export interface LinkState { target: string; where: string; name: string; ok: boolean; missing: boolean }
const props = defineProps<{ st: LinkState }>();
const listing = useListing();
const okDir = folderState(() => props.st.where);
const name = ref<HTMLInputElement | null>(null);

const taken = () => namesIn(clean(props.st.where)).has(props.st.name.trim());
const self = () => join(clean(props.st.where), props.st.name.trim()) === props.st.target.trim();
watchEffect(() => {
  const st = props.st, target = st.target.trim(), n = st.name.trim();
  st.ok = !!okDir.value && !!n && !taken() && !self() && target.startsWith('/') && !n.includes('/');
  const there = listing.lists.get(parent(target));
  st.missing = !!target && !!there && !there.items.some((k) => k.name === split(target).pop());
});
watchEffect(() => { const target = props.st.target.trim(); if (target.startsWith('/')) listing.load(parent(target)).catch(() => {}); });
onMounted(() => setTimeout(() => name.value?.select(), 20));
</script>

<template>
  <div class="well glim-well bk-form" @keydown.enter="pressPrimary">
    <Frow :label="t('props.pointsTo')" :tip="t('props.pointsToTip')"><input id="l-target" v-model="st.target" class="glim-input grow glim-input--mono" spellcheck="false" autocomplete="off"></Frow>
    <Frow :label="t('props.lies')"><PathField id="l-where" v-model="st.where" /></Frow>
    <Frow :label="t('props.name')"><input id="l-name" ref="name" v-model="st.name" class="glim-input grow" spellcheck="false" autocomplete="off"></Frow>
    <Frow :label="t('props.yields')">
      <span id="l-final" class="bk-finalw">
        <span v-if="okDir === false" class="pill fault">{{ t('ui.noFolder') }}</span>
        <span v-else-if="!st.name.trim()" class="pill fault">{{ t('ui.noName') }}</span>
        <span v-else-if="taken()" class="pill fault">{{ t('props.takenThere') }}</span>
        <span v-else-if="self()" class="pill fault">{{ t('props.self') }}</span>
        <FinalPath v-else :path="join(clean(st.where), st.name.trim())" />
      </span>
    </Frow>
  </div>
</template>

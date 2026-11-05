<script setup lang="ts">
// The preview bar and the quick look: the same renderers, compact, followed by the file's details.
import { computed, ref } from 'vue';
import { fmtSize, fmtTime } from '@/core/format';
import { t } from '@/i18n';
import { type FileNode, viewKind } from './kinds';
import Stage from './Stage.vue';
import Frow from '@/ui/Frow.vue';

const props = defineProps<{ n: FileNode; folder: string; autoplay?: boolean }>();
const kind = computed(() => viewKind(props.n));
const info = ref('');
const sibs = computed(() => [props.n]);
</script>

<template>
  <div class="cv-compact" :class="`cvk-${kind}`">
    <Stage :n="n" :folder="folder" :sibs="sibs" compact :autoplay="autoplay" @info="info = $event" />
  </div>
  <div class="well glim-well cv-details">
    <div class="eyebrow">{{ n.name }}</div>
    <Frow :label="t('viewer.details.size')" :value="fmtSize(n.size) || '0 B'" />
    <Frow v-if="info" :label="kind === 'dir' ? t('viewer.details.content') : t('viewer.details.format')" :value="info" />
    <Frow :label="t('viewer.details.changed')" :value="fmtTime(n.time)" />
    <Frow :label="t('viewer.details.owner')" :value="`${n.owner || 'nobody'}:${n.group || 'users'}`" />
    <Frow :label="t('viewer.details.rights')" :value="n.mode || '0664'" />
  </div>
</template>

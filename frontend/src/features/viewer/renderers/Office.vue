<script setup lang="ts">
// Office files: in the office suite when the server has one, else the built-in previews. Formats those cannot read
// fall back to the pages the server draws, and to a short note when it draws none either.
import { EmptyState } from '@glimstone/vue';
import { computed, onMounted, ref } from 'vue';
import { t } from '@/i18n';
import { office } from '../content';
import { ext, joinPath } from '../kinds';
import Docx from './Docx.vue';
import OfficeFrame from './OfficeFrame.vue';
import Pages from './Pages.vue';
import Sheet from './Sheet.vue';
import Slides from './Slides.vue';
import type { REmits, RProps } from './types';

const props = defineProps<RProps>();
const emit = defineEmits<REmits>();

const BUILT_IN: Record<string, typeof Docx> = { docx: Docx, xlsx: Sheet, csv: Sheet, tsv: Sheet, pptx: Slides };
const how = ref<'wait' | 'suite' | 'own' | 'pages' | 'none'>('wait');
const own = computed(() => BUILT_IN[ext(props.n.name)]);
const inner = ref<{ keys?: (e: KeyboardEvent, c: string) => boolean | void } | null>(null);

const fallback = () => { how.value = how.value === 'suite' && own.value ? 'own' : how.value === 'pages' ? 'none' : 'pages'; };

onMounted(async () => {
  const setup = await office();
  how.value = setup.kind !== 'none' && !props.compact ? 'suite' : own.value ? 'own' : 'pages';
});
defineExpose({ keys: (e: KeyboardEvent, c: string) => inner.value?.keys?.(e, c) });
</script>

<template>
  <OfficeFrame v-if="how === 'suite'" :path="joinPath(folder, n.name)" @failed="fallback" />
  <component :is="own" v-else-if="how === 'own'" ref="inner" v-bind="props" @info="emit('info', $event)" @failed="how = 'pages'" />
  <Pages v-else-if="how === 'pages'" ref="inner" v-bind="props" @info="emit('info', $event)" @failed="fallback" />
  <EmptyState v-else-if="how === 'none'" nested :title="t('viewer.office.none')" />
</template>

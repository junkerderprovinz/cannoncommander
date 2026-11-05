<script lang="ts">
import { EmptyState, Pill, Toggle } from '@glimstone/vue';
import { ref } from 'vue';

const wrap = ref(false);
</script>

<script setup lang="ts">
import { computed, onMounted, ref as vref } from 'vue';
import { t } from '@/i18n';
import { fsText } from '../content';
import { ext, joinPath, langOf, plural, viewKind } from '../kinds';
import { highlight } from '../markdown';
import type { REmits, RProps } from './types';

const props = defineProps<RProps>();
const emit = defineEmits<REmits>();

const all = vref<string[] | null>(null);
const truncated = vref(false);
const failed = vref(false);
const shown = computed(() => (props.compact ? all.value?.slice(0, 160) : all.value) || []);
const kind = viewKind(props.n), e = ext(props.n.name);

onMounted(async () => {
  try {
    const got = await fsText(joinPath(props.folder, props.n.name), props.compact ? 64 * 1024 : undefined);
    all.value = got.text.replace(/\n$/, '').split('\n');
    truncated.value = !!got.truncated;
    emit('info', `${langOf(props.n)}, ${plural(all.value.length, 'lines')}${got.truncated ? ', ' + t('viewer.text.cut') : ''}`);
  } catch {
    failed.value = true;
  }
});
</script>

<template>
  <div v-if="!compact" class="cv-bar">
    <Pill v-if="truncated">{{ t('viewer.text.truncated') }}</Pill><span class="grow"></span>
    <span class="cv-swl">{{ t('viewer.text.wrap') }} <Toggle id="cv-wrap" v-model="wrap" /></span>
  </div>
  <EmptyState v-if="failed" nested :title="t('viewer.failed')" />
  <div v-else class="cv-code" :class="{ wrap: wrap || compact }">
    <div class="cv-lines"><div v-for="(l, k) in shown" :key="k" class="ln" v-html="highlight(l, kind, e) || ' '"></div></div>
  </div>
</template>

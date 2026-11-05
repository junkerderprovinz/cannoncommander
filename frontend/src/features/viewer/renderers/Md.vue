<script lang="ts">
import { segItems } from '@/ui/kit';
import { EmptyState, Selector } from '@glimstone/vue';
import { ref } from 'vue';

const mode = ref('md');
</script>

<script setup lang="ts">
import { computed, onMounted, ref as vref } from 'vue';
import { t } from '@/i18n';
import { fsText } from '../content';
import { joinPath, plural } from '../kinds';
import { md } from '../markdown';
import type { REmits, RProps } from './types';

const props = defineProps<RProps>();
const emit = defineEmits<REmits>();
const text = vref<string | null>(null);
const failed = vref(false);
const lines = computed(() => (text.value ?? '').replace(/\n$/, '').split('\n'));

onMounted(async () => {
  try {
    text.value = (await fsText(joinPath(props.folder, props.n.name), props.compact ? 64 * 1024 : undefined)).text;
    emit('info', `Markdown, ${plural(lines.value.length, 'lines')}`);
  } catch {
    failed.value = true;
  }
});
</script>

<template>
  <EmptyState v-if="failed" nested :title="t('viewer.failed')" />
  <article v-else-if="compact" class="cv-md compact" v-html="md(text ?? '')"></article>
  <template v-else>
    <div class="cv-bar"><span class="grow"></span><Selector id="cv-mdv" v-model="mode" :items="segItems([['md', t('viewer.md.rendered')], ['src', t('viewer.md.source')]])" size="sm" activation="manual" inline button-height /></div>
    <div class="cv-scroll">
      <article v-if="mode === 'md'" class="cv-md" v-html="md(text ?? '')"></article>
      <div v-else class="cv-code wrap"><div class="cv-lines"><div v-for="(l, k) in lines" :key="k" class="ln">{{ l || ' ' }}</div></div></div>
    </div>
  </template>
</template>

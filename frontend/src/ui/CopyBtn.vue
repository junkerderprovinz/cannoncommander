<script setup lang="ts">
import { G } from '@/core/glyphs';
import { t } from '@/i18n';
import { copyText } from './kit';

const props = defineProps<{ text: string; tip?: string; done?: string }>();
</script>

<template>
  <button class="btn icon bk-ghost" type="button" :data-bk-copy="text" :data-tip="tip || t('ui.copy')" :aria-label="tip || t('ui.copy')" @click.stop="copyText(props.text, done)"><G n="IconCopy" /></button>
</template>

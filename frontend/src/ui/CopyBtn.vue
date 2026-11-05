<script setup lang="ts">
import { glyphComponent } from '@/core/glyphs';
import { IconButton } from '@glimstone/vue';
import { t } from '@/i18n';
import { copyText } from './kit';

const props = defineProps<{ text: string; tip?: string; done?: string }>();
</script>

<template>
  <IconButton :glyph="glyphComponent('IconCopy')" :tip="tip || t('ui.copy')" :data-bk-copy="text" @click="copyText(props.text, done)" />
</template>

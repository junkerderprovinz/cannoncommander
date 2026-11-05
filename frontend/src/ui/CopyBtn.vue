<script setup lang="ts">
import { t } from '@/i18n';
import BareBtn from './BareBtn.vue';
import { copyText } from './kit';

const props = defineProps<{ text: string; tip?: string; done?: string }>();
</script>

<template>
  <BareBtn g="IconCopy" :tip="tip || t('ui.copy')" :data-bk-copy="text" @click="copyText(props.text, done)" />
</template>

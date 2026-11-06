<script setup lang="ts">
// A size, or for a folder the index has not measured yet a quiet mark, since 0 B would read as an empty folder.
import { fmtSize } from '@/core/format';
import { t } from '@/i18n';

defineProps<{ k: { size: number; sizeUnknown?: boolean } }>();
</script>

<template>
  <span v-if="k.sizeUnknown" class="muted" data-unmeasured :data-tip="t('files.unmeasuredTip')" :aria-label="t('files.unmeasured')">…</span>
  <template v-else>{{ fmtSize(k.size) || '0 B' }}</template>
</template>

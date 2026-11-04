<script setup lang="ts">
import { fmtSize } from '@/core/format';
import { G } from '@/core/glyphs';
import { t } from '@/i18n';

defineProps<{ size: number }>();
</script>

<template>
  <div class="xdel"><span class="xbin"><G n="IconShieldOn" /></span><div><b>{{ t('transfer.parityAsk') }}</b><span class="glim-num">{{ t('transfer.parityText', { size: fmtSize(size) || '0 B' }) }}</span></div></div>
</template>

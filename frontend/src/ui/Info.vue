<script setup lang="ts">
import { G } from '@/core/glyphs';

defineProps<{ tip: string }>();
</script>

<template>
  <span class="info" :data-tip="tip"><G n="IconInfo" /></span>
</template>

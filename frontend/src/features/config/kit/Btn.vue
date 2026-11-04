<script setup lang="ts">
import { G } from '@/core/glyphs';

defineProps<{ label: string; g?: string }>();
</script>

<template>
  <button class="btn"><G v-if="g" :n="g" />{{ label }}</button>
</template>

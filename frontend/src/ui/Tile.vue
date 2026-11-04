<script setup lang="ts">
import { G, glyphOf } from '@/core/glyphs';

defineProps<{ k?: { name: string; dir?: boolean }; g?: string }>();
</script>

<template>
  <span class="bk-tile" :class="{ dir: k?.dir }"><G :n="g || glyphOf(k!)" /></span>
</template>

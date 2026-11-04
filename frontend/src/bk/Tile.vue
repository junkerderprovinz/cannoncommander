<script setup lang="ts">
import { G, glyphOf } from '@/core/glyphs';

defineProps<{ n: { name: string; dir?: boolean } }>();
</script>

<template>
  <span class="bk-tile" :class="{ dir: n.dir }"><G :n="glyphOf(n)" /></span>
</template>

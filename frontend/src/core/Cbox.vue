<script setup lang="ts">
// Stand-in for GlimStone's Checkbox until @glimstone/vue ships one.
import { G } from './glyphs';

defineProps<{ on?: boolean }>();
</script>

<template>
  <span class="cbox" :class="{ on }"><G n="IconCheck" /></span>
</template>

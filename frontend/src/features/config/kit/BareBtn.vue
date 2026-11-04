<script setup lang="ts">
// A row button with no ground of its own. An empty slot keeps the other buttons where they were.
import { G } from '@/core/glyphs';

defineProps<{ g?: string; tip?: string }>();
</script>

<template>
  <button v-if="g" class="btn icon a-bare" :data-tip="tip" :aria-label="tip" @click.stop><G :n="g" /></button>
  <span v-else class="a-slot"></span>
</template>

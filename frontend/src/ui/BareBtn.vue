<script setup lang="ts">
// A row button with no ground of its own, a glyph alone in every label mode so the row keeps its text. An empty slot
// keeps the other buttons where they were.
import { IconTipButton } from '@glimstone/vue';
import { G } from '@/core/glyphs';
import './texts';

defineProps<{ g?: string; tip?: string }>();
</script>

<template>
  <IconTipButton v-if="g" class="a-bare" :tip="tip ?? ''" @click.stop><G :n="g" /></IconTipButton>
  <span v-else class="a-slot"></span>
</template>

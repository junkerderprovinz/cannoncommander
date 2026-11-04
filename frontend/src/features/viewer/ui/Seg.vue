<script setup lang="ts">
// The prototype's segmented control: one pressed button out of a few.
import { G } from '@/core/glyphs';

defineProps<{ opts: [value: string, label: string, glyph?: string][] }>();
const value = defineModel<string>({ required: true });
</script>

<template>
  <div class="seg" role="group">
    <button v-for="[v, label, g] in opts" :key="v" :aria-pressed="v === value" :data-v="v" @click="value = v"><G v-if="g" :n="g" />{{ label }}</button>
  </div>
</template>

<script setup lang="ts">
// A label on the left and a pill with the value on the right, as the details cards of the prototype draw them.
import { G } from '@/core/glyphs';

defineProps<{ label: string; value?: string; tip?: string; pill?: string }>();
</script>

<template>
  <div class="frow">
    <span class="lab"><slot name="label">{{ label }}</slot><span v-if="tip" class="info" :data-tip="tip"><G n="IconInfo" /></span></span>
    <slot><span class="pill glim-num" :class="pill">{{ value }}</span></slot>
  </div>
</template>

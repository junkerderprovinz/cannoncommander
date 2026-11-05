<script setup lang="ts">
// A list row: glyph tile, the name, badges on the right, and buttons that come in on hover.
import { InfoBubble } from '@glimstone/vue';

defineProps<{ name: string; tip?: string | null; mono?: boolean }>();
</script>

<template>
  <div class="a-row" tabindex="0" @keydown.enter.self="($event.target as HTMLElement).click()">
    <span class="tile"><slot name="tile" /></span>
    <span class="a-name" :class="{ mono }"><span class="t">{{ name }}</span><InfoBubble v-if="tip" :tip="tip" /></span>
    <span class="a-badges"><slot name="badges" /></span>
    <span class="a-acts"><slot name="acts" /></span>
  </div>
</template>

<script setup lang="ts">
// Prototype: GlimStone's window frame without its backdrop, so many can stand side by side.
import { Badge, Button } from '@glimstone/vue';
import { glyphComponent } from '@/core/glyphs';

export interface FootButton { label: string; g?: string; primary?: boolean }
defineProps<{ title: string; foot: FootButton[]; wide?: boolean }>();
</script>

<template>
  <div class="glim-modal-card glim-window proto-win" :class="{ 'glim-window--wide': wide }">
    <h2 class="glim-window-title"><Badge tone="heading" size="heading" wrap :inset-start="5">{{ title }}</Badge></h2>
    <div class="glim-modal-body glim-window-body"><slot /></div>
    <div class="glim-modal-foot">
      <span v-if="$slots.start" class="glim-window-start"><slot name="start" /></span>
      <div class="glim-window-actions">
        <Button v-for="(b, i) in foot" :key="i" :label="b.label" :label-key="null" :glyph="b.g ? glyphComponent(b.g) : undefined" :tone="b.primary ? 'accent' : 'neutral'" />
      </div>
    </div>
  </div>
</template>

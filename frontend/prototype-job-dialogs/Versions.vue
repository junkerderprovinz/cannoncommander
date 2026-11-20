<script setup lang="ts">
// Prototype: the two versions of a name that clash, side by side, with the values that differ called out.
import { computed } from 'vue';
import { G } from '@/core/glyphs';
import type { Clash, Texts } from './data';

const props = defineProps<{ clash: Clash; t: Texts }>();
const sides = computed(() => {
  const { fresh, there } = props.clash;
  return [
    { key: 'fresh', role: props.t.newSide, v: fresh, bigger: fresh.bytes > there.bytes, newer: fresh.when > there.when },
    { key: 'there', role: props.t.oldSide, v: there, bigger: there.bytes > fresh.bytes, newer: there.when > fresh.when },
  ];
});
const glyph = computed(() => ({ image: 'IconFileImage', video: 'IconFileVideo', folder: 'IconFolder' })[props.clash.kind]);
</script>

<template>
  <div class="vs">
    <article v-for="s in sides" :key="s.key" class="vs-card" :class="`vs-card--${s.key}`">
      <div class="vs-pic" :class="`vs-pic--${clash.kind} vs-pic--${s.key}`">
        <G v-if="clash.kind !== 'image'" :n="glyph" />
        <span class="vs-role">{{ s.role }}</span>
      </div>
      <dl class="vs-facts">
        <div :class="{ win: s.bigger }"><dt>{{ t.size }}</dt><dd class="glim-num">{{ s.v.size }}<span v-if="s.bigger" class="vs-tag">{{ t.bigger }}</span></dd></div>
        <div :class="{ win: s.newer }"><dt>{{ t.changed }}</dt><dd class="glim-num">{{ s.v.date }}<span v-if="s.newer" class="vs-tag">{{ t.newer }}</span></dd></div>
      </dl>
    </article>
  </div>
</template>

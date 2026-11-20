<script setup lang="ts">
// Prototype: where a job goes, drawn as two ends and a thin path. While it runs, data travels along the path.
import { G } from '@/core/glyphs';

defineProps<{ from: string; to: string; toGlyph?: string; moving?: boolean; paused?: boolean; compact?: boolean }>();
</script>

<template>
  <div class="flow" :class="{ moving, paused, compact }">
    <div class="flow-end"><span class="flow-dot"><G n="IconFolder" /></span><span class="flow-name">{{ from }}</span></div>
    <div class="flow-path" aria-hidden="true">
      <span class="flow-line"></span>
      <span class="flow-rest"></span>
      <span v-for="k in 3" :key="k" class="flow-lane" :style="{ '--lane': k - 1 }"><i></i></span>
    </div>
    <div class="flow-end flow-end--to"><span class="flow-dot"><G :n="toGlyph ?? 'IconFolderOpen'" /></span><span class="flow-name">{{ to }}</span></div>
  </div>
</template>

<script setup lang="ts">
// A client's mark, its resting version and, where the brand has one, the single colour version a lit tile swaps in.
// The other client draws the link glyph, and a key made for no known client the key glyph.
import { computed, useId } from 'vue';
import { G } from '@/core/glyphs';
import type { McpClient } from './clients';
import { CLIENT_MARKS } from './marks';
import './mcp.css';

const props = defineProps<{ client?: McpClient; forKey?: boolean }>();
const suffix = useId().replace(/[^a-zA-Z0-9]/g, '');

// The same mark sits on a tile, on key rows and in a window at once, and a second copy of a gradient's id would point
// every copy at the first one.
const own = (svg: string) => svg
  .replace(/id="([^"]+)"/g, `id="$1-${suffix}"`)
  .replace(/url\(#([^)]+)\)/g, `url(#$1-${suffix})`)
  .replace(/href="#([^"]+)"/g, `href="#$1-${suffix}"`);

const html = computed(() => {
  const mark = props.client?.mark ? CLIENT_MARKS[props.client.mark] : undefined;
  if (!mark) return '';
  const rest = own(mark.rest);
  return mark.hover ? rest.replace('<svg ', '<svg class="glim-mark-rest" ') + mark.hover.replace('<svg ', '<svg class="glim-mark-hover" ') : rest;
});
</script>

<template>
  <span v-if="html" class="mcp-mark" v-html="html"></span>
  <G v-else :n="forKey || !client ? 'IconKey' : 'IconLink'" />
</template>

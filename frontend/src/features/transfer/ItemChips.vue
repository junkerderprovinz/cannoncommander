<script setup lang="ts">
// The selection as a row of chips: the first few by name, the rest as a count.
import { fmtSize } from '@/core/format';
import { G, glyphOf } from '@/core/glyphs';
import type { Entry } from '@/files/entries';
import { plural } from '@/i18n';
import { sizeOf } from './paths';

withDefaults(defineProps<{ items: Entry[]; max?: number }>(), { max: 4 });
</script>

<template>
  <div class="xchips">
    <span v-for="k in items.slice(0, max)" :key="(k.at || '') + k.name" class="xchip"><G :n="glyphOf(k)" /><span>{{ k.name }}</span><small class="glim-num">{{ fmtSize(sizeOf(k)) || '0 B' }}</small></span>
    <span v-if="items.length > max" class="xchip more">{{ plural(items.length - max, 'transfer.more') }}</span>
  </div>
</template>

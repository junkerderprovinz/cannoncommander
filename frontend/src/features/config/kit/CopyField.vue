<script setup lang="ts">
// A read-only value with a copy button beside it, in the prototype's field so it lines up with the other controls.
import { G } from '@/core/glyphs';
import { c, copy } from './kit';

defineProps<{ value: string; grow?: boolean }>();
</script>

<template>
  <span class="a-ctl" :class="{ grow }"><input class="field w mono" readonly :value="value"><button class="btn icon" :data-copy="value" :data-tip="c('copy')" @click.stop="copy(value)"><G n="IconCopy" /></button></span>
</template>

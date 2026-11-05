<script setup lang="ts">
// The round mark on a row picked for an action on many (rule 39).
import { G } from './glyphs';

defineProps<{ on?: boolean }>();
</script>

<template>
  <span class="rowmark" :class="{ on }"><G n="IconCheck" /></span>
</template>

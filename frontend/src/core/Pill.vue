<script setup lang="ts">
// Stand-in for GlimStone's Pill until @glimstone/vue ships one.
defineProps<{ tone?: 'ok' | 'warn' | 'fault' | 'state' }>();
</script>

<template>
  <span class="pill" :class="tone"><slot /></span>
</template>

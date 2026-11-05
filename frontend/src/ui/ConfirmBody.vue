<script setup lang="ts">
defineProps<{ text: string[] }>();
</script>

<template>
  <div class="well glim-well a-text"><p v-for="(p, k) in text" :key="k">{{ p }}</p></div>
</template>

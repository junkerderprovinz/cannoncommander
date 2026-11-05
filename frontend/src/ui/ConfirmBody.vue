<script setup lang="ts">
import { SubCard } from '@glimstone/vue';
defineProps<{ text: string[] }>();
</script>

<template>
  <SubCard class="a-text"><p v-for="(p, k) in text" :key="k">{{ p }}</p></SubCard>
</template>

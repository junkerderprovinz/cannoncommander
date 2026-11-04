<script setup lang="ts">
import WinFrame from './WinFrame.vue';
import { winStack } from './windows';
</script>

<template>
  <Teleport to="body">
    <WinFrame v-for="w in winStack" :key="w.id" :w="w" />
  </Teleport>
</template>

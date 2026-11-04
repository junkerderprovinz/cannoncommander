<script setup lang="ts">
// The prototype's switch, a button with role switch.
const on = defineModel<boolean>({ required: true });
</script>

<template>
  <button class="sw" role="switch" :aria-checked="on" @click="on = !on"></button>
</template>

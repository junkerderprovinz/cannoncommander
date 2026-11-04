<script setup lang="ts">
// The switch every setting uses. GlimStone's Toggle draws its knob and label differently, so this keeps the
// prototype's markup until the look is settled.
const model = defineModel<boolean>({ required: true });
</script>

<template>
  <button class="sw" role="switch" :aria-checked="!!model" @click="model = !model"></button>
</template>

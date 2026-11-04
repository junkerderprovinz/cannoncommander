<script setup lang="ts">
// The prototype's switch; GlimStone's Toggle draws its knob and label differently.
const model = defineModel<boolean>({ required: true });
</script>

<template>
  <button class="sw" type="button" role="switch" :aria-checked="!!model" @click="model = !model"></button>
</template>

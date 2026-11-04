<script setup lang="ts">
// A settings card: the title in the notch and, where the card explains itself, a hint beside it.
import Info from './Info.vue';

defineProps<{ title: string; hint?: string }>();
</script>

<template>
  <section class="card glim-card"><span class="notch">{{ title }}<template v-if="hint">{{ ' ' }}<Info :tip="hint" /></template></span><div class="card-body"><slot /></div></section>
</template>

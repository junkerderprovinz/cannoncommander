<script setup lang="ts">
// One labelled row of a card or window: the label with its hint on the left, the control on the right.
import Info from './Info.vue';

defineProps<{ label?: string; tip?: string; cls?: string }>();
</script>

<template>
  <div class="frow" :class="cls"><span class="lab"><slot name="label">{{ label }}</slot><template v-if="tip">{{ ' ' }}<Info :tip="tip" /></template></span><slot /></div>
</template>

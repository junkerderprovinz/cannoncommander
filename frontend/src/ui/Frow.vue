<script setup lang="ts">
// One labelled row of a card or window: the label with its hint on the left, the control on the right, which takes
// its accessible name from the label. A row with only a value to show gets it as a pill.
import { InfoBubble } from '@glimstone/vue';
import { provide } from 'vue';
import { rowLabel } from './kit';

const props = defineProps<{ label?: string; tip?: string; value?: string; pill?: string }>();
provide(rowLabel, () => props.label ?? '');
</script>

<template>
  <div class="frow"><span class="lab"><slot name="label">{{ label }}</slot><template v-if="tip">{{ ' ' }}<InfoBubble :tip="tip" /></template></span><slot><span v-if="value !== undefined" class="pill glim-num" :class="pill">{{ value }}</span></slot></div>
</template>

<script setup lang="ts">
// The colour dot and the tags beside a name in the file list.
import { computed } from 'vue';
import { type Labelled, useLabels } from '@/stores/labels';

const props = defineProps<{ k: Labelled; path: string }>();
const labels = useLabels();
const l = computed(() => labels.of(props.k, props.path));
const colour = computed(() => labels.colour(l.value.colour));
</script>

<template>
  <i v-if="colour" class="cdot" :style="{ '--c': colour.hex }" :title="labels.colourName(colour)"></i>
  <span v-for="tag in l.tags || []" :key="tag" class="pill tag">#{{ tag }}</span>
</template>

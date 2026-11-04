<script setup lang="ts">
// A moon on the tree row of a sleeping disk.
import { computed } from 'vue';
import { G } from '@/core/glyphs';
import { t } from '@/i18n';
import { diskName } from '@/features/jobs/label';
import { useUnraid } from './state';

const props = defineProps<{ dev: string }>();
const U = useUnraid();
const asleep = computed(() => U.asleep(props.dev));
</script>

<template>
  <i v-if="asleep" class="u-moon" :data-tip="t('unraid.moonTip', { disk: diskName(dev) })"><G n="IconMoon" /></i>
</template>

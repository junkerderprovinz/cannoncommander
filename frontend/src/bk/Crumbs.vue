<script setup lang="ts">
// A folder as a row of steps from the server down, each a button that goes there.
import { computed } from 'vue';
import { split } from '@/files/paths';
import { t } from '@/i18n';
import './kit';

const props = defineProps<{ path: string; attr: string }>();
const emit = defineEmits<{ go: [path: string] }>();
const parts = computed(() => split(props.path));
const upTo = (k: number) => '/' + parts.value.slice(0, k + 1).join('/');
</script>

<template>
  <button class="pseg" :class="{ last: !parts.length }" v-bind="{ [attr]: '/' }" @click="emit('go', '/')">{{ t('bk.server') }}</button>
  <template v-for="(p, k) in parts" :key="k"><span class="psep" aria-hidden="true">›</span><button class="pseg" :class="{ last: k === parts.length - 1 }" v-bind="{ [attr]: upTo(k) }" @click="emit('go', upTo(k))">{{ p.replace('::', '') }}</button></template>
</template>

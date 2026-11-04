<script setup lang="ts">
// "Cache" on an entry the mover still has to carry to the array. Only shares that empty their pool get it, since
// elsewhere a file on the cache is where it belongs.
import { computed, watchEffect } from 'vue';
import type { Entry } from '@/files/entries';
import { join } from '@/files/paths';
import { t } from '@/i18n';
import { diskName } from '@/features/jobs/label';
import { useListing } from '@/stores/listing';
import { useUnraid } from './state';
import { moverPool, poolPhrase } from './store';

const props = defineProps<{ k: Entry; path: string }>();
const U = useUnraid();
const folder = computed(() => props.k.at || props.path);
const pool = computed(() => {
  const w = U.place(folder.value);
  return !props.k.up && w?.vol === 'user' ? moverPool(U.share(w.share || props.k.name)) : '';
});
watchEffect(() => {
  if (pool.value) useListing().wantMeta([join(folder.value, props.k.name)], ['disk']);
});
const waits = computed(() => !!pool.value && !!U.devicesOf(folder.value, props.k.name)?.includes(pool.value));
</script>

<template>
  <span v-if="waits" class="pill u-cache" :data-tip="t(k.dir ? 'unraid.cacheDir' : 'unraid.cacheFile', { pool: poolPhrase(pool) })">{{ pool === 'cache' ? t('unraid.cache') : diskName(pool) }}</span>
</template>

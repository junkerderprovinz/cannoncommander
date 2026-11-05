<script setup lang="ts">
// A disk image: what kind, its name, how it starts, its partitions or files, and whether a checksum lies beside it.
import { FieldRow } from '@glimstone/vue';
import { computed, onMounted, ref } from 'vue';
import { api } from '@/api/client';
import { fmtSize } from '@/core/format';
import { G } from '@/core/glyphs';
import { t } from '@/i18n';
import { useListing } from '@/stores/listing';
import { fsStat, type ImageInfo } from '../content';
import { ext, joinPath } from '../kinds';
import type { REmits, RProps } from './types';

const props = defineProps<RProps>();
const emit = defineEmits<REmits>();
const listing = useListing();

const path = computed(() => joinPath(props.folder, props.n.name));
const iso = ext(props.n.name) === 'iso';
const image = ref<ImageInfo>({});
const inside = ref<{ name: string; dir: boolean }[]>([]);
const format = computed(() => image.value.format || t(iso ? 'viewer.disk.iso' : 'viewer.disk.raw'));
const hasSum = computed(() => (listing.lists.get(props.folder)?.items || []).some((k) => k.name === props.n.name + '.sha256'));
const parts = computed(() => image.value.parts || []);

onMounted(async () => {
  emit('info', format.value);
  image.value = (await fsStat(path.value).catch(() => null))?.image || {};
  emit('info', format.value);
  if (iso && !parts.value.length) inside.value = (await api.fsList(path.value + '::').catch(() => null))?.items.map((k) => ({ name: k.name + (k.dir ? '/' : ''), dir: k.dir })) || [];
});
</script>

<template>
  <div class="cv-scroll cv-disk">
    <div class="well glim-well">
      <div class="eyebrow">{{ t('viewer.disk.eyebrow') }}</div>
      <FieldRow :label="t('viewer.disk.format')" :value="format" />
      <FieldRow v-if="image.label" :label="t('viewer.disk.label')" :value="image.label" />
      <FieldRow v-if="image.boot" :label="t('viewer.disk.boot')" :value="image.boot" />
      <FieldRow v-if="image.used != null" :label="t('viewer.disk.used')" :value="fmtSize(image.used)" :hint="t('viewer.disk.usedTip')" />
      <FieldRow :label="t('viewer.disk.sum')" :value="hasSum ? t('viewer.disk.sumThere') : t('viewer.disk.sumNone')" :tone="hasSum ? 'ok' : 'neutral'" :hint="hasSum ? t('viewer.disk.sumTip') : ''" />
    </div>
    <div v-if="parts.length" class="well glim-well">
      <div class="eyebrow">{{ t('viewer.disk.parts') }}</div>
      <div class="cv-parts"><i v-for="(p, k) in parts" :key="k" :class="'p' + (k % 4)" :style="{ flex: Math.max(p.size / n.size, 0.03) }"></i></div>
      <FieldRow v-for="(p, k) in parts" :key="k" :label="p.label" :value="fmtSize(p.size)">
        <template #label><span class="cv-key" :class="'p' + (k % 4)"></span>{{ p.label }}</template>
      </FieldRow>
    </div>
    <div v-else-if="inside.length" class="well glim-well">
      <div class="eyebrow">{{ t('viewer.disk.content') }}</div>
      <div class="cv-mini">
        <div v-for="x in inside" :key="x.name" class="cv-mrow"><span class="tile"><G :n="x.dir ? 'IconFolder' : 'IconFile'" /></span><span class="t mono">{{ x.name }}</span><span></span></div>
      </div>
    </div>
  </div>
</template>

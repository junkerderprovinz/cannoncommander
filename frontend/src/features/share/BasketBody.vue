<script setup lang="ts">
import { EmptyState, IconButton, Pill, SubCard } from '@glimstone/vue';
import { computed } from 'vue';
import { fmtSize } from '@/core/format';
import { G, glyphOf, glyphComponent } from '@/core/glyphs';
import { parent } from '@/files/paths';
import { t } from '@/i18n';
import { plural } from '@/ui/kit';
import { basket } from './basket';

const folders = computed(() => new Set(basket.map((x) => parent(x.path))).size);
const size = computed(() => fmtSize(basket.reduce((a, x) => a + x.size, 0)) || '0 B');
const drop = (path: string) => basket.splice(basket.findIndex((x) => x.path === path), 1);
</script>

<template>
  <SubCard id="cs-bbody">
    <template v-if="basket.length">
      <div class="glim-eyebrow">{{ t('share.basketFrom', { objects: plural(basket.length, 'ui.objects'), folders: plural(folders, 'share.folders'), size }) }}</div>
      <div class="cs-basket">
        <div v-for="x in basket" :key="x.path" class="cs-brow"><span class="tile"><G :n="glyphOf(x)" /></span><span class="t">{{ x.name }}</span><span class="muted mono cs-bdir">{{ parent(x.path) }}</span>
          <Pill class="glim-num">{{ fmtSize(x.size) || '0 B' }}</Pill><IconButton :tip="t('share.takeOut')" :glyph="glyphComponent('IconClose')" :data-bdel="x.path" @click="drop(x.path)" /></div>
      </div>
    </template>
    <EmptyState v-else nested :glyph="glyphComponent('IconCollector')" :title="t('share.empty')" :hint="t('share.emptyTip')" />
  </SubCard>
</template>

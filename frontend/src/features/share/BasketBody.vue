<script setup lang="ts">
import { computed } from 'vue';
import { fmtSize } from '@/core/format';
import { G, glyphOf } from '@/core/glyphs';
import { parent } from '@/files/paths';
import { t } from '@/i18n';
import { plural } from '@/ui/kit';
import { basket } from './basket';

const folders = computed(() => new Set(basket.map((x) => parent(x.path))).size);
const size = computed(() => fmtSize(basket.reduce((a, x) => a + x.size, 0)) || '0 B');
const drop = (path: string) => basket.splice(basket.findIndex((x) => x.path === path), 1);
</script>

<template>
  <div id="cs-bbody" class="well">
    <template v-if="basket.length">
      <div class="eyebrow">{{ t('share.basketFrom', { objects: plural(basket.length, 'ui.objects'), folders: plural(folders, 'share.folders'), size }) }}</div>
      <div class="cs-basket">
        <div v-for="x in basket" :key="x.path" class="cs-brow"><span class="tile"><G :n="glyphOf(x)" /></span><span class="t">{{ x.name }}</span><span class="muted mono cs-bdir">{{ parent(x.path) }}</span>
          <span class="pill glim-num">{{ fmtSize(x.size) || '0 B' }}</span><button class="btn icon" type="button" :data-bdel="x.path" :data-tip="t('share.takeOut')" :aria-label="t('share.takeOut')" @click="drop(x.path)"><G n="IconClose" /></button></div>
      </div>
    </template>
    <div v-else class="empty"><G n="IconCollector" /><div>{{ t('share.empty') }}</div><div class="muted" style="font-size: 12px">{{ t('share.emptyTip') }}</div></div>
  </div>
</template>

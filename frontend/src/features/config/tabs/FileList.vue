<script setup lang="ts">
import { computed } from 'vue';
import { run } from '@/core/actions';
import { DAY, fmtDate, fmtTime, now } from '@/core/format';
import { G } from '@/core/glyphs';
import { prefs, setPref } from '@/core/prefs';
import Card from '../kit/Card.vue';
import Frow from '../kit/Frow.vue';
import { ago, c, P } from '../kit/kit';
import Seg from '../kit/Seg.vue';
import Sw from '../kit/Sw.vue';

const sample = now() - 2 * DAY;
// The preview panel lives in the second window, so turning it on turns both windows on.
const quickview = computed({ get: () => prefs.quickview, set: (on: boolean) => { if (on) prefs.dual = true; setPref('quickview', on); } });
const foldersFirst = computed({ get: () => prefs.foldersFirst !== false, set: (on: boolean) => setPref('foldersFirst', on) });
</script>

<template>
  <Card :title="c('list.list')">
    <Frow :label="c('list.hidden')" :tip="c('list.hiddenTip')"><Sw v-model="P.showHidden" /></Frow>
    <Frow :label="c('list.sizes')" :tip="c('list.sizesTip')"><Sw v-model="P.folderSizes" /></Frow>
    <Frow :label="c('list.foldersFirst')" :tip="c('list.foldersFirstTip')"><Sw v-model="foldersFirst" /></Frow>
    <Frow :label="c('list.date')"><Seg v-model="P.dateFormat" :options="[['short', fmtDate(sample)], ['long', fmtTime(sample)], ['relative', ago(sample)]]" /></Frow>
    <Frow :label="c('list.density')"><Seg v-model="P.density" :options="[['compact', c('list.compact'), 'IconRowCompact'], ['medium', c('list.medium'), 'IconRowMedium'], ['comfortable', c('list.comfortable'), 'IconRowComfortable']]" /></Frow>
    <Frow :label="c('list.columns')" :tip="c('list.columnsTip')"><button class="btn" data-run="columns" @click="run('columns')"><G n="IconSliders" />{{ c('list.chooseColumns') }}</button></Frow>
  </Card>
  <Card :title="c('list.bars')">
    <Frow :label="c('list.side')" :tip="c('list.sideTip')"><Sw v-model="P.side" /></Frow>
    <Frow :label="c('list.fkeys')" :tip="c('list.fkeysTip')"><Sw v-model="P.fkeys" /></Frow>
    <Frow :label="c('list.quickview')" :tip="c('list.quickviewTip')"><Sw id="a-qv" v-model="quickview" /></Frow>
  </Card>
  <Card :title="c('list.refresh')">
    <Frow :label="c('list.live')" :tip="c('list.liveTip')"><Sw v-model="P.liveRefresh" data-pref="liveRefresh" /></Frow>
  </Card>
</template>

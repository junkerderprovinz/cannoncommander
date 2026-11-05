<script setup lang="ts">
import { segItems } from '@/ui/kit';
import { Button, Card, FieldRow, Selector, Toggle } from '@glimstone/vue';
import { computed } from 'vue';
import { run } from '@/core/actions';
import { DAY, fmtDate, fmtTime, now } from '@/core/format';
import { prefs, setPref } from '@/core/prefs';
import { ago, c, P } from '@/features/config/shared';

const sample = now() - 2 * DAY;
// The preview panel lives in the second window, so turning it on turns both windows on.
const quickview = computed({ get: () => prefs.quickview, set: (on: boolean) => { if (on) prefs.dual = true; setPref('quickview', on); } });
const foldersFirst = computed({ get: () => prefs.foldersFirst !== false, set: (on: boolean) => setPref('foldersFirst', on) });
</script>

<template>
  <Card :title="c('list.list')">
    <FieldRow :label="c('list.hidden')" :hint="c('list.hiddenTip')"><Toggle v-model="P.showHidden" /></FieldRow>
    <FieldRow :label="c('list.sizes')" :hint="c('list.sizesTip')"><Toggle v-model="P.folderSizes" /></FieldRow>
    <FieldRow :label="c('list.foldersFirst')" :hint="c('list.foldersFirstTip')"><Toggle v-model="foldersFirst" /></FieldRow>
    <FieldRow :label="c('list.date')"><Selector v-model="P.dateFormat" :items="segItems([['short', fmtDate(sample)], ['long', fmtTime(sample)], ['relative', ago(sample)]])" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    <FieldRow :label="c('list.density')"><Selector v-model="P.density" :items="segItems([['compact', c('list.compact'), 'IconRowCompact'], ['medium', c('list.medium'), 'IconRowMedium'], ['comfortable', c('list.comfortable'), 'IconRowComfortable']])" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    <FieldRow :label="c('list.columns')" :hint="c('list.columnsTip')"><Button :label="c('list.chooseColumns')" label-key="IconSliders" data-run="columns" @click="run('columns')" /></FieldRow>
  </Card>
  <Card :title="c('list.bars')">
    <FieldRow :label="c('list.side')" :hint="c('list.sideTip')"><Toggle v-model="P.side" /></FieldRow>
    <FieldRow :label="c('list.fkeys')" :hint="c('list.fkeysTip')"><Toggle v-model="P.fkeys" /></FieldRow>
    <FieldRow :label="c('list.quickview')" :hint="c('list.quickviewTip')"><Toggle id="a-qv" v-model="quickview" /></FieldRow>
  </Card>
  <Card :title="c('list.refresh')">
    <FieldRow :label="c('list.live')" :hint="c('list.liveTip')"><Toggle v-model="P.liveRefresh" data-pref="liveRefresh" /></FieldRow>
  </Card>
</template>

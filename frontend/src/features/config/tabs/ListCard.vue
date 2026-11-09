<script setup lang="ts">
// How the file list looks, which Aussehen shows as Explorer shows it under its view.
import { segItems } from '@/ui/kit';
import { Button, Card, FieldRow, Selector, Toggle } from '@glimstone/vue';
import { computed } from 'vue';
import { run } from '@/core/actions';
import { DAY, fmtDate, fmtTime, now } from '@/core/format';
import { prefs, setPref } from '@/core/prefs';
import { ago, c, P } from '@/features/config/shared';

const sample = now() - 2 * DAY;
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
</template>

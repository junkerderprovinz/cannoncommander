<script setup lang="ts">
import { Card, FieldRow, Toggle } from '@glimstone/vue';
import { computed } from 'vue';
import { prefs, setPref } from '@/core/prefs';
import { c, P } from '@/features/config/shared';

// The preview panel lives in the second window, so turning it on turns both windows on.
const quickview = computed({ get: () => prefs.quickview, set: (on: boolean) => { if (on) prefs.dual = true; setPref('quickview', on); } });
</script>

<template>
  <Card :title="c('list.bars')">
    <FieldRow :label="c('list.side')" :hint="c('list.sideTip')"><Toggle v-model="P.side" /></FieldRow>
    <FieldRow :label="c('list.fkeys')" :hint="c('list.fkeysTip')"><Toggle v-model="P.fkeys" /></FieldRow>
    <FieldRow :label="c('list.quickview')" :hint="c('list.quickviewTip')"><Toggle id="a-qv" v-model="quickview" /></FieldRow>
  </Card>
  <Card :title="c('list.refresh')">
    <FieldRow :label="c('list.live')" :hint="c('list.liveTip')"><Toggle v-model="P.liveRefresh" data-pref="liveRefresh" /></FieldRow>
  </Card>
</template>

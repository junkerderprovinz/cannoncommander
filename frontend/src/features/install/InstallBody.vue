<script setup lang="ts">
// How to put CannonCommander on the home screen or the taskbar, with the way of the browser at hand first in mind.
import { Pill, SubCard } from '@glimstone/vue';
import { G } from '@/core/glyphs';
import { t } from '@/i18n';

const WAYS = [['chrome', 'IconBrowser'], ['android', 'IconPhone'], ['ios', 'IconPhone'], ['firefox', 'IconBrowser']] as const;
const mine = () => {
  const ua = navigator.userAgent;
  return /iPhone|iPad|iPod/.test(ua) ? 'ios' : /Firefox/.test(ua) ? 'firefox' : /Android/.test(ua) ? 'android' : 'chrome';
};
const at = mine();
// The app logo on its white tile, as the home screen will show it.
const icon = `${import.meta.env.BASE_URL}icons/favicon.svg`;
</script>

<template>
  <SubCard><div class="in-head"><span class="in-icon"><img :src="icon" alt=""></span><div><b>{{ t('install.head') }}</b><div class="muted">{{ t('install.intro') }}</div></div></div></SubCard>
  <SubCard :title="t('install.how')">
    <div v-for="[id, g] in WAYS" :key="id" class="in-way" :class="{ cur: id === at }"><G :n="g" /><div><b>{{ t(`install.ways.${id}.head`) }}</b><Pill v-if="id === at">{{ t('install.yours') }}</Pill><div class="muted">{{ t(`install.ways.${id}.how`) }}</div></div></div>
  </SubCard>
</template>

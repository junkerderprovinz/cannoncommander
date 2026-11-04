<script setup lang="ts">
// How to put CannonCommander on the home screen or the taskbar, with the way of the browser at hand first in mind.
import { computed } from 'vue';
import { glyph, G } from '@/core/glyphs';
import { prefs } from '@/core/prefs';
import { t } from '@/i18n';

const WAYS = [['chrome', 'IconBrowser'], ['android', 'IconPhone'], ['ios', 'IconPhone'], ['firefox', 'IconBrowser']] as const;
const mine = () => {
  const ua = navigator.userAgent;
  return /iPhone|iPad|iPod/.test(ua) ? 'ios' : /Firefox/.test(ua) ? 'firefox' : /Android/.test(ua) ? 'android' : 'chrome';
};
// The folder glyph in dark ink on the accent colour, as the installed app's icon shows it.
const icon = computed(() => {
  const d = glyph('IconFolderOpen');
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512"><rect width="512" height="512" rx="112" fill="${prefs.accent}"/><svg x="116" y="116" width="280" height="280" viewBox="${d.viewBox}" fill="#161616" color="#161616">${d.svg}</svg></svg>`;
  return `data:image/svg+xml,${encodeURIComponent(svg)}`;
});
const at = mine();
</script>

<template>
  <div class="well"><div class="in-head"><span class="in-icon"><img :src="icon" alt=""></span><div><b>{{ t('install.head') }}</b><div class="muted">{{ t('install.intro') }}</div></div></div></div>
  <div class="well"><div class="eyebrow">{{ t('install.how') }}</div>
    <div v-for="[id, g] in WAYS" :key="id" class="in-way" :class="{ cur: id === at }"><G :n="g" /><div><b>{{ t(`install.ways.${id}.head`) }}</b><span v-if="id === at" class="pill">{{ t('install.yours') }}</span><div class="muted">{{ t(`install.ways.${id}.how`) }}</div></div></div>
  </div>
</template>

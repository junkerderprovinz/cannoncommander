<script setup lang="ts">
// The other forms of CannonCommander as the README's download buttons. None is out yet, so each says it is still to come.
import { BrandMark, Card, ReadmeButton } from '@glimstone/vue';
import { ANDROID_SVG, APPLE_SVG, FDROID_SVG, LINUX_SVG, PLAY_SVG, WINDOWS_SVG, type ReadmeBrand } from '@glimstone/core';
import { c } from '@/features/config/shared';

interface Build { brand: ReadmeBrand; name: string; svg: string; markClass?: string }
const desktop: Build[] = [
  { brand: 'windows', name: 'Windows', svg: WINDOWS_SVG, markClass: 'glim-windows-mark' },
  { brand: 'apple', name: 'macOS', svg: APPLE_SVG },
  { brand: 'linux', name: 'Linux', svg: LINUX_SVG },
];
const phone: Build[] = [
  { brand: 'play', name: 'Google Play', svg: PLAY_SVG },
  { brand: 'fdroid', name: 'F-Droid', svg: FDROID_SVG },
  { brand: 'android', name: 'Android', svg: ANDROID_SVG, markClass: 'glim-android-mark' },
];
const cards = [
  { title: 'builds.desktop', hint: 'builds.desktopHint', builds: desktop },
  { title: 'builds.phone', hint: 'builds.phoneHint', builds: phone },
];
</script>

<template>
  <Card v-for="card in cards" :key="card.title" :title="c(card.title)" :hint="c(card.hint)">
    <div class="glim-readme-btn-rows">
      <ReadmeButton v-for="b in card.builds" :key="b.brand" :brand="b.brand" :parts="[{ name: b.name }]" :mark-class="b.markClass" :soon-label="c('builds.soon')">
        <template #mark><BrandMark :svg="b.svg" /></template>
      </ReadmeButton>
    </div>
  </Card>
</template>

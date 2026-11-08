<script setup lang="ts">
// The other forms of CannonCommander as the README's download buttons. A release build offers the desktop app of its
// own release, which every release carries; a build between releases cannot know one and says it is still to come.
// The desktop app offers the server instead, as the other apps of the workshop do.
import { BrandMark, Card, ReadmeButton } from '@glimstone/vue';
import { ANDROID_SVG, APPLE_SVG, DOCKER_SVG, FDROID_SVG, LINUX_SVG, PLAY_SVG, UNRAID_SVG, WINDOWS_SVG, ZIP_SVG, type ReadmeBrand, type ReadmePart } from '@glimstone/core';
import { computed, onMounted, ref } from 'vue';
import { admin, type System } from '@/api/admin';
import { c } from '@/features/config/shared';
import { desktopAsset, REPO } from '../support';

interface Build { brand: ReadmeBrand; parts: [ReadmePart, ...ReadmePart[]]; svg: string; markClass?: string }

const sys = ref<System | null>(null);
onMounted(() => { admin.system().then((s) => { sys.value = s; }, () => {}); });

const desktop = computed<Build[]>(() => {
  const at = (file: string) => desktopAsset(sys.value, file);
  return [
    { brand: 'windows', svg: WINDOWS_SVG, markClass: 'glim-windows-mark', parts: [
      { name: 'Windows', sub: 'x64', href: at('windows-amd64-installer.exe') },
      { name: 'Windows', sub: 'Portable', href: at('windows-amd64.zip') },
    ] },
    { brand: 'apple', svg: APPLE_SVG, parts: [{ name: 'macOS', sub: 'Universal', href: at('macos-universal.zip') }] },
    { brand: 'linux', svg: LINUX_SVG, parts: [
      { name: 'Linux', sub: 'x64', href: at('linux-amd64-installer') },
      { name: 'Linux', sub: 'ARM64', href: at('linux-arm64-installer') },
    ] },
  ];
});
// Not in Community Applications yet, so Unraid is still to come.
const server = computed<Build[]>(() => [
  { brand: 'unraid', svg: UNRAID_SVG, parts: [{ name: 'Unraid' }] },
  { brand: 'docker', svg: DOCKER_SVG, markClass: 'glim-docker-mark', parts: [{ name: 'Docker', href: 'https://raw.githubusercontent.com/junkerderprovinz/cannoncommander/main/compose.yaml' }] },
  { brand: 'zip', svg: ZIP_SVG, parts: [{ name: c('builds.source'), href: REPO + '/releases/latest' }] },
]);
const phone: Build[] = [
  { brand: 'play', svg: PLAY_SVG, parts: [{ name: 'Google Play' }] },
  { brand: 'fdroid', svg: FDROID_SVG, parts: [{ name: 'F-Droid' }] },
  { brand: 'android', svg: ANDROID_SVG, markClass: 'glim-android-mark', parts: [{ name: 'Android' }] },
];
const cards = computed(() => [
  sys.value?.deployment === 'desktop'
    ? { title: 'builds.server', hint: 'builds.serverHint', builds: server.value }
    : { title: 'builds.desktop', hint: desktop.value[0]!.parts[0].href ? 'builds.desktopHintOut' : 'builds.desktopHint', builds: desktop.value },
  { title: 'builds.phone', hint: 'builds.phoneHint', builds: phone },
]);
</script>

<template>
  <Card v-for="card in cards" :key="card.title" :title="c(card.title)" :hint="c(card.hint)">
    <div class="glim-readme-btn-rows">
      <ReadmeButton v-for="b in card.builds" :key="b.brand" :brand="b.brand" :parts="b.parts" :mark-class="b.markClass" :soon-label="c('builds.soon')">
        <template #mark><BrandMark :svg="b.svg" /></template>
      </ReadmeButton>
    </div>
  </Card>
</template>

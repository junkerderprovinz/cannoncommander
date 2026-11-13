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

// GlimStone knows no Flatpak brand and draws that button without a tile, so the button brings its own, coloured in
// styles/app.css. The mark is Flatpak's path from Simple Icons, the mark itself CC BY 3.0 (flatpak.org/press).
const FLATPAK_SVG =
  '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M12 0c-.556 0-1.111.144-1.61.432l-7.603 4.39a3.217 3.217 0 0 0-1.61 2.788v8.78c0 1.151.612 2.212 1.61 2.788l7.603 4.39a3.217 3.217 0 0 0 3.22 0l7.603-4.39a3.217 3.217 0 0 0 1.61-2.788V7.61a3.217 3.217 0 0 0-1.61-2.788L13.61.432A3.218 3.218 0 0 0 12 0Zm0 2.358c.15 0 .299.039.431.115l7.604 4.39c.132.077.24.187.315.316L12 12v9.642a.863.863 0 0 1-.431-.116l-7.604-4.39a.866.866 0 0 1-.431-.746V7.61c0-.153.041-.302.116-.43L12 12Z" fill="currentColor"/></svg>';

interface Build { brand: ReadmeBrand | 'flatpak'; tile?: string; parts: [ReadmePart, ...ReadmePart[]]; svg: string; markClass?: string }

const sys = ref<System | null>(null);
onMounted(() => { admin.system().then((s) => { sys.value = s; }, () => {}); });

const desktop = computed<Build[]>(() => {
  const at = (file: string) => desktopAsset(sys.value, file);
  return [
    { brand: 'windows', svg: WINDOWS_SVG, markClass: 'glim-windows-mark', parts: [
      { name: 'Windows', sub: 'x64', href: at('windows-amd64-installer.exe') },
      { name: 'Windows', sub: 'ARM64', href: at('windows-arm64-installer.exe') },
    ] },
    { brand: 'apple', svg: APPLE_SVG, parts: [{ name: 'macOS', sub: 'Universal', href: at('macos-universal.zip') }] },
    { brand: 'linux', svg: LINUX_SVG, parts: [
      { name: 'Linux', sub: 'x64', href: at('linux-amd64-installer') },
      { name: 'Linux', sub: 'ARM64', href: at('linux-arm64-installer') },
    ] },
    { brand: 'flatpak', tile: 'glim-tile-flatpak', svg: FLATPAK_SVG, markClass: 'glim-flatpak-mark', parts: [
      { name: 'Flatpak', sub: 'x64', href: at('linux-amd64.flatpak') },
      { name: 'Flatpak', sub: 'ARM64', href: at('linux-arm64.flatpak') },
    ] },
  ];
});
const server = computed<Build[]>(() => [
  { brand: 'unraid', svg: UNRAID_SVG, parts: [{ name: 'Unraid', href: 'https://ca.unraid.net/apps?q=cannoncommander' }] },
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
      <ReadmeButton v-for="b in card.builds" :key="b.brand" :brand="b.brand as ReadmeBrand" :class="b.tile" :parts="b.parts" :mark-class="b.markClass" :soon-label="c('builds.soon')">
        <template #mark><BrandMark :svg="b.svg" /></template>
      </ReadmeButton>
    </div>
  </Card>
</template>

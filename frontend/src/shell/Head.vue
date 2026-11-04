<script setup lang="ts">
import { computed } from 'vue';
import { run } from '@/core/actions';
import { G } from '@/core/glyphs';
import { closeMenu, menu, menuState } from '@/core/menu';
import { allMenuItems, MENUS, menuName, openMenuBar } from '@/core/menus';
import { prefs } from '@/core/prefs';
import { go, view } from '@/core/views';
import { t } from '@/i18n';
import { useUser } from '@/stores/user';
import { shell } from './state';

const user = useUser();
const files = computed(() => view.id === 'files');
const settings = computed(() => view.id === 'settings');
const initials = computed(() => user.me.name.slice(0, 2).toUpperCase());

const toggleBar = (i: number) => (menuState.bar === i ? closeMenu() : openMenuBar(i));
// While a menu of the bar is open, pointing at another title opens that one instead.
const hoverBar = (i: number) => { if (menuState.anchor && menuState.bar !== i) openMenuBar(i); };
const at = (e: MouseEvent) => (e.currentTarget as HTMLElement).getBoundingClientRect();

function allMenus(e: MouseEvent) {
  const r = at(e);
  menu(r.left, r.bottom + 6, allMenuItems());
}
function account(e: MouseEvent) {
  const r = at(e);
  menu(r.right - 260, r.bottom + 6, [
    { head: `${user.me.full}, ${user.me.role}` },
    { label: t('head.security'), g: 'IconKey', run: () => go('settings', 'security') },
    { label: t('head.activity'), g: 'IconLog', run: () => run('openActivity') },
    '-', 'logout',
  ]);
}
</script>

<template>
  <header class="head glim-card">
    <button class="brand" data-go-view="files" :data-tip="t('head.brand')" :aria-label="t('head.brand')" @click="go('files')"><span class="brand-mark"><G n="IconFolderOpen" /></span></button>
    <span class="hsep"></span>
    <template v-if="files">
      <nav class="menubar" :aria-label="t('menus.label')">
        <button v-for="(_, i) in MENUS" :key="i" :data-menu="i" :class="{ open: menuState.bar === i }" @click="toggleBar(i)" @mouseover="hoverBar(i)">{{ menuName(i) }}</button>
      </nav>
      <button class="btn icon narrow-only" data-all-menus :data-tip="t('menus.label')" @click="allMenus"><G n="IconMenu" /></button>
    </template>
    <button v-else class="btn hback" data-go-view="files" :data-tip="t('head.backTip')" @click="go('files')"><G n="IconBack" />{{ t('head.back') }}</button>
    <span class="spacer"></span>
    <div v-if="files" class="seg" role="group" :aria-label="t('head.layout')">
      <button data-run="single" :aria-pressed="!prefs.dual" :data-tip="t('head.singleTip')" @click="run('single')"><G n="IconViewSimple" /><span class="wide-only">{{ t('actions.single') }}</span></button>
      <button data-run="dual" :aria-pressed="prefs.dual" :data-tip="t('head.dualTip')" @click="run('dual')"><G n="IconCompare" /><span class="wide-only">{{ t('actions.dual') }}</span></button>
    </div>
    <button class="btn icon" data-run="palette" :data-tip="t('head.palette')" @click="run('palette')"><G n="IconSearch" /></button>
    <button class="btn" data-run="jobs" :data-tip="t('head.jobs')" @click="run('jobs')"><G n="IconQueued" /><span class="wide-only">{{ t('head.jobs') }}</span><span id="job-count" class="count glim-num" :class="{ idle: !shell.jobCount }">{{ shell.jobCount }}</span></button>
    <button class="btn icon" :class="{ on: settings }" :data-go-view="settings ? 'files' : 'settings'" :data-tip="settings ? t('head.settingsClose') : t('head.settings')" :aria-pressed="settings" @click="go(settings ? 'files' : 'settings')"><G n="IconGear" /></button>
    <button class="avatar" data-account :data-tip="user.me.full" @click="account"><i>{{ initials }}</i></button>
  </header>
</template>

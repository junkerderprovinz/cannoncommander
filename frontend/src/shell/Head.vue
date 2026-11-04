<script setup lang="ts">
import { MenuBar } from '@glimstone/vue';
import { computed, onMounted, ref, watch } from 'vue';
import { run } from '@/core/actions';
import { G } from '@/core/glyphs';
import { menu, resolveItems, toGlim } from '@/core/menu';
import { allMenuItems, MENUS, menuItems, menuName, setMenuBarOpener } from '@/core/menus';
import { prefs } from '@/core/prefs';
import { go, view } from '@/core/views';
import { t } from '@/i18n';
import { useUser } from '@/stores/user';
import { shell } from './state';

const user = useUser();
const files = computed(() => view.id === 'files');
const settings = computed(() => view.id === 'settings');
const initials = computed(() => user.me.name.slice(0, 2).toUpperCase());

// The menus are read again whenever an action changes, so a menu opened later shows today's state.
const bar = ref<InstanceType<typeof MenuBar> | null>(null);
const barMenus = computed(() => MENUS.map((_, i) => ({ label: menuName(i), items: toGlim(resolveItems(menuItems(i))) })));
onMounted(() => setMenuBarOpener((i) => bar.value?.open(i)));
// The prototype numbered its menu titles, and its step files still click them by that number.
watch(bar, (b) => {
  (b?.$el as HTMLElement | undefined)?.querySelectorAll<HTMLElement>('.glim-menubar-item').forEach((el, i) => { el.dataset.menu = String(i); });
}, { flush: 'post' });
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
      <MenuBar ref="bar" class="menubar" :label="t('menus.label')" :menus="barMenus" />
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

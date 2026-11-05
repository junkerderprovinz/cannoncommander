<script setup lang="ts">
import { Button, IconButton, MenuBar, Selector } from '@glimstone/vue';
import { computed, onMounted, ref, watch } from 'vue';
import { run } from '@/core/actions';
import { G, glyphComponent } from '@/core/glyphs';
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
const barMenus = computed(() => MENUS.map((_, i) => ({ id: 'menu', label: menuName(i), items: toGlim(resolveItems(menuItems(i))) })));
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
    { head: [user.me.full, user.me.role && t(`config.roles.${user.me.role.toLowerCase()}`)].filter(Boolean).join(', ') },
    { label: t('head.security'), g: 'IconKey', run: () => go('settings', 'security') },
    { label: t('head.activity'), g: 'IconLog', run: () => run('openActivity') },
    '-', 'logout',
  ]);
}
const layouts = computed(() => [
  { id: 'single', label: t('actions.single'), glyph: glyphComponent('IconViewSimple'), tip: t('head.singleTip'), attrs: { 'data-run': 'single' } },
  { id: 'dual', label: t('actions.dual'), glyph: glyphComponent('IconCompare'), tip: t('head.dualTip'), attrs: { 'data-run': 'dual' } },
]);
</script>

<template>
  <header class="head glim-card">
    <button class="brand" data-go-view="files" :data-tip="t('head.brand')" :aria-label="t('head.brand')" @click="go('files')"><span class="brand-mark"><G n="IconFolderOpen" /></span></button>
    <span class="hsep"></span>
    <template v-if="files">
      <MenuBar ref="bar" class="menubar" :label="t('menus.label')" :menus="barMenus" />
      <IconButton class="narrow-only" :tip="t('menus.label')" :glyph="glyphComponent('IconMenu')" data-all-menus @click="allMenus" />
    </template>
    <Button v-else class="hback" :label="t('head.back')" label-key="IconBack" data-go-view="files" :title="t('head.backTip')" @click="go('files')" />
    <span class="spacer"></span>
    <Selector v-if="files" :model-value="prefs.dual ? 'dual' : 'single'" class="head-layout" size="md" activation="manual" inline :label="t('head.layout')" :items="layouts" @update:model-value="run($event)" />
    <IconButton :tip="t('head.palette')" :glyph="glyphComponent('IconSearch')" data-run="palette" @click="run('palette')" />
    <Button :compact-below="900" :label="t('head.jobs')" label-key="IconQueued" data-run="jobs" @click="run('jobs')"><template #after><span id="job-count" class="count glim-num" :class="{ idle: !shell.jobCount }">{{ shell.jobCount }}</span></template></Button>
    <IconButton :tip="settings ? t('head.settingsClose') : t('head.settings')" :glyph="glyphComponent('IconGear')" :data-go-view="settings ? 'files' : 'settings'" :pressed="settings" @click="go(settings ? 'files' : 'settings')" />
    <button class="avatar" data-account :data-tip="user.me.full" @click="account"><i>{{ initials }}</i></button>
  </header>
</template>

<script setup lang="ts">
import { Button, MenuBar } from '@glimstone/vue';
import { computed, h, onMounted, ref, watch } from 'vue';
import { run } from '@/core/actions';
import { glyphComponent } from '@/core/glyphs';
import { menu, resolveItems, toGlim } from '@/core/menu';
import { allMenuItems, MENUS, menuItems, menuName, setMenuBarOpener } from '@/core/menus';
import { prefs } from '@/core/prefs';
import { go, view } from '@/core/views';
import { t } from '@/i18n';
import { useUser } from '@/stores/user';
import { HEAD_LOGO } from './logo';
import { shell } from './state';

const user = useUser();
const files = computed(() => view.id === 'files');
const initials = computed(() => user.me.name.slice(0, 2).toUpperCase());
// The avatar is a button of the same family as the others, with the initials where they have a glyph.
const Initials = () => h('span', { class: 'initials', 'aria-hidden': 'true' }, initials.value);

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
    'palette', '-', 'settings',
    { label: t('head.security'), g: 'IconKey', run: () => go('settings', 'security') },
    { label: t('head.activity'), g: 'IconLog', run: () => run('openActivity') },
    '-', 'logout',
  ]);
}
// One button for the layout: its glyph and its words say what a press does, and it stays pressed while both panels show.
const flip = computed(() => (prefs.dual
  ? { id: 'single', glyph: 'IconViewSimple', tip: t('head.singleTip') }
  : { id: 'dual', glyph: 'IconCompare', tip: t('head.dualTip') }));
</script>

<template>
  <header class="head glim-card">
    <button class="brand" data-go-view="files" :data-tip="t('head.brand')" :aria-label="t('head.brand')" @click="go('files')">
      <span class="brand-logo" :style="{ aspectRatio: HEAD_LOGO.ratio }"><svg class="brand-cannon" :viewBox="HEAD_LOGO.viewBox" aria-hidden="true" v-html="HEAD_LOGO.cannon" /><svg class="brand-ball" :viewBox="HEAD_LOGO.viewBox" aria-hidden="true" v-html="HEAD_LOGO.ball" /></span>
    </button>
    <span class="hsep"></span>
    <template v-if="files">
      <MenuBar ref="bar" class="menubar" :label="t('menus.label')" :menus="barMenus" />
      <Button class="narrow-only" compact :label="t('menus.label')" :label-key="null" :glyph="glyphComponent('IconMenu')" data-all-menus @click="allMenus" />
    </template>
    <Button v-else class="hback" :label="t('head.back')" label-key="IconBack" data-go-view="files" :title="t('head.backTip')" @click="go('files')" />
    <span class="spacer"></span>
    <Button v-if="files" class="head-layout" compact :label="flip.tip" :label-key="null" :glyph="glyphComponent(flip.glyph)" :aria-pressed="prefs.dual" :data-run="flip.id" @click="run(flip.id)" />
    <Button :compact-below="900" :label="t('head.jobs')" label-key="IconQueued" data-run="jobs" @click="run('jobs')"><template #after><span id="job-count" class="count glim-num" :class="{ idle: !shell.jobCount }">{{ shell.jobCount }}</span></template></Button>
    <Button class="avatar" compact :label="user.me.full" :label-key="null" :glyph="Initials" data-account @click="account" />
  </header>
</template>

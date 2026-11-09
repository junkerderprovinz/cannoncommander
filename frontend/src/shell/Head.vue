<script setup lang="ts">
import { Button, MenuBar } from '@glimstone/vue';
import { computed, h, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { actions, run } from '@/core/actions';
import { glyphComponent } from '@/core/glyphs';
import { menu, resolveItems, toGlim } from '@/core/menu';
import { allMenuItems, MENUS, menuItems, menuName, setMenuBarOpener } from '@/core/menus';
import { prefs } from '@/core/prefs';
import { go, view } from '@/core/views';
import { language, plural, t } from '@/i18n';
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
// The jobs live in the avatar's menu, so the avatar carries their count while any run.
function account(e: MouseEvent) {
  const r = at(e);
  menu(r.right - 260, r.bottom + 6, [
    { head: [user.me.full, user.me.role && t(`config.roles.${user.me.role.toLowerCase()}`)].filter(Boolean).join(', ') },
    'palette', '-',
    { ...actions.jobs!, count: shell.jobCount || undefined },
    { label: t('head.activity'), g: 'IconLog', run: () => run('openActivity') },
    '-', 'logout',
  ]);
}
// One button for the layout: its glyph and its words name the layout that shows and the bubble says what a press does.
// It is not a pressed toggle, since its name already changes with the layout and a reader would hear the state twice.
// How much room the menus and the words on the buttons need depends on the language, so the head folds by the room it
// has: first its buttons keep only their glyphs, then the menus fold into one button. Each step remembers the width the
// head needed before it, and comes undone once the head is that wide again.
const head = ref<HTMLElement | null>(null);
const fold = ref(0);
let needed: number[] = [];
function fit() {
  const el = head.value!;
  if (el.scrollWidth > el.clientWidth + 1 && fold.value < 2) {
    needed[fold.value] = el.scrollWidth;
    fold.value++;
    void nextTick(fit);
  } else if (fold.value > 0 && el.clientWidth >= needed[fold.value - 1]!) {
    fold.value--;
    void nextTick(fit);
  }
}
let watcher: ResizeObserver | undefined;
onMounted(() => {
  watcher = new ResizeObserver(fit);
  watcher.observe(head.value!);
});
onBeforeUnmount(() => watcher?.disconnect());
watch([language, files], () => {
  fold.value = 0;
  needed = [];
  void nextTick(fit);
});

const layout = computed(() => (prefs.dual
  ? { id: 'single', label: t('actions.dual'), glyph: 'IconPaneTwo', tip: t('head.toSingle') }
  : { id: 'dual', label: t('actions.single'), glyph: 'IconPaneOne', tip: t('head.toDual') }));
</script>

<template>
  <header ref="head" class="head glim-card" :class="{ 'fold-menus': fold > 1 }">
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
    <Button v-if="files" class="head-layout" :compact="fold > 0" :compact-below="900" :label="layout.label" :label-key="null" :glyph="glyphComponent(layout.glyph)" :title="layout.tip" :data-run="layout.id" @click="run(layout.id)" />
    <Button :compact="fold > 0" :compact-below="900" :label="t('head.settings')" label-key="IconGear" data-run="settings" @click="run('settings')" />
    <Button class="avatar" compact :label="user.me.full" :label-key="null" :glyph="Initials" :title="shell.jobCount ? plural(shell.jobCount, 'head.running') : undefined" data-account @click="account"><template #after><span id="job-count" class="count glim-num" :class="{ idle: !shell.jobCount }" aria-hidden="true">{{ shell.jobCount }}</span></template></Button>
  </header>
</template>

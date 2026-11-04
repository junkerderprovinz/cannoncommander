<script setup lang="ts">
// Stand-in for GlimStone's context menu and its side menu. Both live on body, so the menu's scrolling cannot clip the side menu.
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { choose, closeMenu, closeSub, isHead, type MenuItem, menuState, openSub, type Placed } from './menu';
import MenuList from './MenuList.vue';
import { stepMenu } from './menus';

const menuEl = ref<HTMLElement | null>(null);
const subEl = ref<HTMLElement | null>(null);
const pos = ref<Placed>({ left: 0, top: 0 });
const subPos = ref<Placed>({ left: 0, top: 0 });

const enabled = (el: HTMLElement | null) => [...(el?.querySelectorAll<HTMLButtonElement>('button:not([disabled])') ?? [])];
const box = (): Placed => {
  const r = menuEl.value!.getBoundingClientRect();
  return { left: r.left, top: r.top };
};

watch(() => menuState.list, async (list) => {
  if (!list) return;
  pos.value = { left: menuState.x, top: menuState.y };
  await nextTick();
  const m = menuEl.value;
  if (!m) return;
  const r = m.getBoundingClientRect();
  pos.value = menuState.at ?? {
    left: Math.max(8, Math.min(menuState.x, innerWidth - r.width - 8)),
    top: menuState.above ? Math.max(8, menuState.y - r.height - 8) : Math.min(menuState.y, innerHeight - r.height - 8),
  };
  const want = menuState.focusK >= 0 ? m.querySelector<HTMLElement>(`[data-k="${menuState.focusK}"]`) : enabled(m)[0];
  // A menu whose items are all still unbuilt holds the focus itself, so the arrows still walk the menu bar.
  (want ?? m).focus();
});

watch(() => menuState.sub, async (sub) => {
  if (!sub) return;
  await nextTick();
  const host = menuEl.value?.querySelector<HTMLElement>(`[data-k="${sub.host}"]`), s = subEl.value;
  if (!host || !s) return;
  const hr = host.getBoundingClientRect(), sr = s.getBoundingClientRect();
  subPos.value = {
    left: hr.right + sr.width + 6 > innerWidth ? hr.left - sr.width - 6 : hr.right + 6,
    top: Math.max(8, Math.min(hr.top - 6, innerHeight - sr.height - 8)),
  };
  const want = sub.focusK >= 0 ? s.querySelector<HTMLElement>(`[data-k="${sub.focusK}"]`) : sub.focus ? enabled(s)[0] : null;
  want?.focus();
});

const hit = (e: Event) => (e.target as Element).closest<HTMLButtonElement>('[data-k]');

function onClick(e: MouseEvent) {
  const b = hit(e);
  if (!b || b.disabled) return;
  const k = +b.dataset.k!, it = menuState.list![k];
  if (!it || it === '-') return;
  if (!isHead(it) && it.sub) return openSub(k, true);
  choose(it as MenuItem, k, box());
}

function onSubClick(e: MouseEvent) {
  const b = hit(e);
  if (!b || b.disabled) return;
  const k = +b.dataset.k!, it = menuState.sub!.list[k];
  if (it && it !== '-') choose(it as MenuItem, k, box(), true);
}

function onOver(e: MouseEvent) {
  const b = (e.target as Element).closest<HTMLElement>('button[data-k]');
  if (!b) return;
  if (b.hasAttribute('data-sub')) {
    if (menuState.sub?.host !== +b.dataset.k!) openSub(+b.dataset.k!);
  } else if (menuState.sub) closeSub();
}

function cycle(e: KeyboardEvent, el: HTMLElement | null) {
  const bs = enabled(el), i = bs.indexOf(document.activeElement as HTMLButtonElement);
  if (e.key === 'ArrowDown') { e.preventDefault(); bs[(i + 1) % bs.length]?.focus(); }
  if (e.key === 'ArrowUp') { e.preventDefault(); bs[(i - 1 + bs.length) % bs.length]?.focus(); }
}

function onKey(e: KeyboardEvent) {
  cycle(e, menuEl.value);
  const a = document.activeElement as HTMLElement | null;
  if (e.key === 'ArrowRight' && a?.hasAttribute('data-sub')) { e.preventDefault(); openSub(+a.dataset.k!, true); return; }
  if (e.key === 'ArrowRight' && menuState.anchor) { e.preventDefault(); stepMenu(1); }
  if (e.key === 'ArrowLeft' && menuState.anchor) { e.preventDefault(); stepMenu(-1); }
}

function onSubKey(e: KeyboardEvent) {
  cycle(e, subEl.value);
  if (e.key === 'ArrowLeft') {
    e.preventDefault();
    const host = menuState.sub!.host;
    closeSub();
    menuEl.value?.querySelector<HTMLElement>(`[data-k="${host}"]`)?.focus();
  }
}

// Capture phase, so a button that opens a menu still sees the old one gone before it draws its own.
function outside(e: MouseEvent) {
  const t = e.target as Element;
  if (!t.closest?.('#menu, #submenu') && !t.closest?.('.menubar')) closeMenu();
}
onMounted(() => document.addEventListener('click', outside, true));
onBeforeUnmount(() => document.removeEventListener('click', outside, true));
</script>

<template>
  <Teleport to="body">
    <div v-if="menuState.list" id="menu" ref="menuEl" class="menu glim-card" role="menu" tabindex="-1"
      :style="{ left: pos.left + 'px', top: pos.top + 'px' }" @click="onClick" @mouseover="onOver" @keydown="onKey">
      <MenuList :list="menuState.list" :open-k="menuState.sub?.host ?? -1" />
    </div>
    <div v-if="menuState.list && menuState.sub" id="submenu" ref="subEl" class="menu glim-card" role="menu"
      :style="{ left: subPos.left + 'px', top: subPos.top + 'px' }" @click="onSubClick" @keydown="onSubKey">
      <MenuList :list="menuState.sub.list" />
    </div>
  </Teleport>
</template>

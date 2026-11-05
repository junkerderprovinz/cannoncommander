<script setup lang="ts">
import { computed, watchEffect } from 'vue';
import { run } from '@/core/actions';
import { extend } from '@/core/extend';
import { fmtSize } from '@/core/format';
import { G } from '@/core/glyphs';
import { menu } from '@/core/menu';
import { Render } from '@/core/Render';
import { cancelBtn, win } from '@/core/windows';
import { t } from '@/i18n';
import type { Bookmark, TreeItem } from '@/api/types';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';
import { useTrash } from '@/stores/trash';
import { useUser } from '@/stores/user';
import { moveBookmark, renameBookmark, unpin } from './bookmarks';
import { folderItems } from './menus';

const F = usePanels();
const listing = useListing();
const user = useUser();
const trash = useTrash();
const cur = computed(() => F.T().path);

const rows = computed(() => {
  const out: { k: TreeItem; depth: number; open: boolean }[] = [];
  const walk = (path: string, depth: number) => {
    for (const k of listing.trees.get(path) ?? []) {
      const open = F.openTree.has(k.path);
      out.push({ k, depth, open });
      if (open) walk(k.path, depth + 1);
    }
  };
  walk('/', 0);
  return out;
});

watchEffect(() => {
  for (const p of ['/', ...F.openTree]) if (!listing.trees.has(p)) listing.tree(p).catch(() => {});
});

const glyphOf = (k: TreeItem, open: boolean) => k.path === '/mnt' ? 'IconDatabase' : k.path === '/boot' ? 'IconFlash' : k.path === '/mnt/remotes' ? 'IconCloud'
  : k.remote ? 'IconNetwork' : k.free ? 'IconZFS' : open ? 'IconFolderOpen' : 'IconFolder';
const toggle = (p: string) => (F.openTree.has(p) ? F.openTree.delete(p) : F.openTree.add(p));

const esc = (s: string) => s.replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c]!);
function rename(b: Bookmark) {
  win({
    title: t('side.renameTitle'),
    body: `<div class="well glim-well"><div class="frow"><span class="lab">${esc(t('side.name'))}</span><input class="glim-input m" id="bmn" value="${esc(b.name)}"></div></div>`,
    foot: [cancelBtn(), { label: t('side.rename'), g: 'IconPencil', primary: true, run: (el) => {
      const name = el.querySelector<HTMLInputElement>('#bmn')!.value.trim();
      if (!name) return false;
      renameBookmark(b, name);
      return true;
    } }],
  });
}

function bookmarkMenu(e: MouseEvent, b: Bookmark) {
  const items = folderItems(b.path);
  menu(e.clientX, e.clientY, [
    items[0]!, items[2]!, items[3]!,
    '-',
    { label: t('side.rename'), g: 'IconPencil', run: () => rename(b) },
    { label: t('side.moveUp'), g: 'IconMoveUp', run: () => moveBookmark(b, -1) },
    { label: t('side.moveDown'), g: 'IconMoveDown', run: () => moveBookmark(b, 1) },
    '-',
    { label: t('side.unpin'), g: 'IconUnlink', run: () => unpin(b) },
  ]);
}
</script>

<template>
  <aside id="side" class="side">
    <section class="side-card">
      <span class="notch">{{ t('side.bookmarks') }}</span>
      <button class="ptab add side-add" data-run="addBookmark" :data-tip="t('side.addBookmark')" @click="run('addBookmark')"><G n="IconAdd" /></button>
      <div id="shortcuts">
        <div v-for="(s, i) in user.bookmarks" :key="s.path" class="nav-row" :class="{ cur: cur === s.path }" :data-go="s.path" :data-bm="i" @click="F.go(s.path)" @contextmenu.prevent.stop="bookmarkMenu($event, s)">
          <G :n="s.g" /><span>{{ s.name }}</span><button class="unpin" :data-unpin="i" :data-tip="t('side.unpin')" :aria-label="t('side.unpinLabel', { name: s.name })" @click.stop="unpin(s)"><G n="IconPin" /></button>
        </div>
        <div class="nav-row" data-run="openTrash" @click="run('openTrash')"><G n="IconTrash" /><span>{{ t('side.trash') }}</span><span class="meta glim-num">{{ trash.items.length }}</span></div>
        <Render v-for="(row, k) in extend.sideRows" :key="k" :node="row()" />
      </div>
    </section>
    <section class="side-card side-tree">
      <span class="notch">{{ t('side.tree') }}</span>
      <div id="tree">
        <div v-for="{ k, depth, open } in rows" :key="k.path" class="nav-row tree-row" :class="{ cur: cur === k.path }" :style="{ paddingLeft: `${0.3 + depth * 0.85}rem` }"
          :data-go="k.path" :data-tip="k.remote" @click="F.go(k.path)" @contextmenu.prevent.stop="menu($event.clientX, $event.clientY, folderItems(k.path))">
          <button v-if="k.hasDirs" class="tree-tog" :class="{ open }" :data-tog="k.path" :aria-label="open ? t('side.collapse') : t('side.expand')" @click.stop="toggle(k.path)"><G n="IconExpand" /></button>
          <span v-else class="tree-tog"></span>
          <G :n="glyphOf(k, open)" /><span>{{ k.label || k.name }}</span>
          <Render v-for="(dec, n) in extend.treeDecorators" :key="n" :node="dec(k.path)" />
          <span v-if="k.free" class="meta glim-num">{{ t('files.free', { size: fmtSize(k.free) }) }}</span>
        </div>
      </div>
    </section>
  </aside>
</template>

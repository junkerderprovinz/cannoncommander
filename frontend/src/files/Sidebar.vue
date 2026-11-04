<script setup lang="ts">
import { computed, watchEffect } from 'vue';
import { run } from '@/core/actions';
import { extend } from '@/core/extend';
import { fmtSize } from '@/core/format';
import { G } from '@/core/glyphs';
import { menu } from '@/core/menu';
import { Render } from '@/core/Render';
import { toast } from '@/core/toast';
import { t } from '@/i18n';
import type { Bookmark, TreeItem } from '@/api/types';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';
import { useUser } from '@/stores/user';
import { folderItems } from './menus';

const F = usePanels();
const listing = useListing();
const user = useUser();
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

function unpin(b: Bookmark) {
  const i = user.bookmarks.indexOf(b);
  user.bookmarks.splice(i, 1);
  toast(t('side.unpinned', { name: b.name }), 'ok', { label: t('core.undo'), run: () => { user.bookmarks.splice(i, 0, b); } });
}

function move(b: Bookmark, d: number) {
  const i = user.bookmarks.indexOf(b), to = i + d;
  if (to < 0 || to >= user.bookmarks.length) return;
  user.bookmarks.splice(to, 0, user.bookmarks.splice(i, 1)[0]!);
}

function bookmarkMenu(e: MouseEvent, b: Bookmark) {
  menu(e.clientX, e.clientY, [
    ...folderItems(b.path),
    '-',
    { label: t('side.moveUp'), g: 'IconMoveUp', run: () => move(b, -1) },
    { label: t('side.moveDown'), g: 'IconMoveDown', run: () => move(b, 1) },
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
        <div class="nav-row" data-run="openTrash" @click="run('openTrash')"><G n="IconTrash" /><span>{{ t('side.trash') }}</span><span class="meta glim-num">{{ user.trash.length }}</span></div>
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
          <G :n="glyphOf(k, open)" /><span>{{ k.label || k.name }}</span><span v-if="k.free" class="meta glim-num">{{ t('files.free', { size: fmtSize(k.free) }) }}</span>
          <Render v-for="(dec, n) in extend.treeDecorators" :key="n" :node="dec(k.path)" />
        </div>
      </div>
    </section>
  </aside>
</template>

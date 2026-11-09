<script setup lang="ts">
// The remotes in a card of their own, each with its state and a pin that puts it among the shortcuts or takes it out
// again. A remote that is not mounted reads quieter, and for an admin a click mounts it first.
import { Card, SidebarRow } from '@glimstone/vue';
import { computed } from 'vue';
import { admin, type Remote } from '@/api/admin';
import { G, glyphComponent } from '@/core/glyphs';
import { host } from '@/core/host';
import { menu } from '@/core/menu';
import { toast } from '@/core/toast';
import { go } from '@/core/views';
import { pin, unpin } from '@/files/bookmarks';
import { folderItems } from '@/files/menus';
import { t } from '@/i18n';
import { usePanels } from '@/stores/panels';
import { useUser } from '@/stores/user';
import { reloadRemotes, remotes } from './store';

const F = usePanels(), user = useUser();
const shown = computed(() => !host.desktop && remotes.list.length > 0);
const pinnedAs = (r: Remote) => user.bookmarks.find((b) => b.path === r.mount);
const state = (r: Remote) => (r.error ? t('remotes.failed') : r.mounted ? '' : t('remotes.unmounted'));
const tip = (r: Remote) => [r.mount, r.error ? t('remotes.failedTip', { error: r.error }) : r.mounted ? t('remotes.mountedTip') : t('remotes.unmountedTip')].join('\n');

function togglePin(r: Remote) {
  const b = pinnedAs(r);
  if (b) unpin(b);
  else pin(r.name, r.mount, 'IconCloud');
}

async function mount(r: Remote, on: boolean) {
  try {
    await admin.mountRemote(r.name, on);
    toast(t(on ? 'remotes.nowMounted' : 'remotes.nowUnmounted', { name: r.name }));
  } catch (e) {
    toast(e instanceof Error ? e.message : String(e), 'warn');
  }
  await reloadRemotes();
}

async function open(r: Remote) {
  if (!r.mounted && !user.admin()) return toast(t('remotes.askAdmin', { name: r.name }), 'warn');
  if (!r.mounted) await mount(r, true);
  if (remotes.list.find((x) => x.name === r.name)?.mounted) F.go(r.mount);
}

function rowMenu(e: MouseEvent, r: Remote) {
  const items = r.mounted ? folderItems(r.mount).slice(0, 1) : [{ label: t('remotes.open'), g: 'IconFolderOpen', run: () => open(r) }];
  menu(e.clientX, e.clientY, [
    ...items,
    pinnedAs(r) ? { label: t('remotes.unpin'), g: 'IconUnlink', run: () => togglePin(r) } : { label: t('remotes.pin'), g: 'IconPin', run: () => togglePin(r) },
    ...(user.admin() ? ['-' as const,
      r.mounted ? { label: t('remotes.unmount'), g: 'IconUnlink', run: () => mount(r, false) } : { label: t('remotes.mount'), g: 'IconLink', run: () => mount(r, true) },
      { label: t('remotes.settings'), g: 'IconGear', run: () => go('settings', 'remotes') }] : []),
  ]);
}
</script>

<template>
  <Card v-if="shown" class="side-card side-remotes" :title="t('remotes.title')">
    <div id="remotes">
      <SidebarRow v-for="r in remotes.list" :key="r.name" :label="r.name" :glyph="glyphComponent('IconCloud')" :active="r.mounted && F.T().path === r.mount"
        :class="{ unmounted: !r.mounted }" :data-remote="r.name" :data-tip="tip(r)" @click="open(r)" @contextmenu.prevent.stop="rowMenu($event, r)">
        <template #after>
          <span v-if="state(r)" class="meta" :class="{ warn: r.error }">{{ state(r) }}</span>
          <button class="unpin" :class="{ pinned: pinnedAs(r) }" :data-pin="r.name" :data-tip="pinnedAs(r) ? t('remotes.unpin') : t('remotes.pin')"
            :aria-label="t(pinnedAs(r) ? 'remotes.unpinLabel' : 'remotes.pinLabel', { name: r.name })" :aria-pressed="!!pinnedAs(r)" @click.stop="togglePin(r)"><G n="IconPin" /></button>
        </template>
      </SidebarRow>
    </div>
  </Card>
</template>

<script setup lang="ts">
// The folder picker's body: a path to type and the server's tree, folders only unless files with one of the given
// endings are wanted.
import { FieldRow, TextField, Tree, type TreeNode } from '@glimstone/vue';
import { onMounted, ref } from 'vue';
import { exists } from '@/api/admin';
import { call } from '@/api/client';
import { glyphComponent } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { t } from '@/i18n';
import { useListing } from '@/stores/listing';
import { clean } from './tools';

const props = defineProps<{ win: WinEntry; start: string; exts?: string[]; done: (p: string) => void }>();

const listing = useListing();
const chosen = ref<string | null>(props.start);
const parts = props.start.split('/').filter(Boolean);
const opened = ref(['/mnt', ...parts.map((_, k) => '/' + parts.slice(0, k + 1).join('/'))]);
const files = new Set<string>();
const ext = (n: string) => (n.includes('.') ? n.split('.').pop()!.toLowerCase() : '');
const join = (dir: string, name: string) => (dir === '/' ? '' : dir) + '/' + name;
const closedGlyph = (path: string, remote: boolean, volume: boolean) => (path === '/boot' ? 'IconFlash' : remote || path === '/remotes' ? 'IconCloud' : volume ? 'IconZFS' : 'IconFolder');

async function load(id: string | null) {
  const path = id ?? '/';
  const tree = await listing.tree(path).catch(() => []);
  const nodes: TreeNode[] = tree.map((k) => {
    const glyph = closedGlyph(k.path, !!k.remote, !!k.total);
    return {
      id: k.path, label: k.label || k.name, attrs: { 'data-pick': k.path },
      // With files wanted, a folder can matter for what lies in it, so every folder may open.
      hasChildren: props.exts ? true : k.hasDirs,
      glyph: glyphComponent(glyph), openGlyph: glyphComponent(glyph === 'IconFolder' ? 'IconFolderOpen' : glyph),
    };
  });
  if (props.exts) {
    const items = (await call<{ items: { name: string; dir: boolean }[] }>('/fs/list?' + new URLSearchParams({ path })).catch(() => null))?.body?.items ?? [];
    for (const f of items) {
      if (f.dir || !props.exts.includes(ext(f.name))) continue;
      const p = join(path, f.name);
      files.add(p);
      nodes.push({ id: p, label: f.name, glyph: glyphComponent('IconFile'), attrs: { 'data-pick': p } });
    }
  }
  return nodes;
}

async function choose() {
  const p = clean(chosen.value ?? '') || '/';
  if (!(await exists(p, props.exts ? 'file' : 'dir'))) return toast(props.exts ? t('ui.needFile') : t('ui.noFolder'), 'warn');
  props.done(p);
  props.win.close();
}

function activate(id: string) {
  if (props.exts && !files.has(id)) return;
  chosen.value = id;
  void choose();
}

onMounted(() => {
  props.win.foot = [cancelBtn(), { label: t('ui.pick'), g: 'IconCheck', primary: true, run: () => { void choose(); return false; } }];
});
</script>

<template>
  <FieldRow :label="exts ? t('ui.file') : t('ui.folder')"><TextField id="a-pick-path" v-model="chosen as string" width="wide" mono /></FieldRow>
  <Tree id="a-tree" v-model="chosen" v-model:open="opened" class="glim-well a-tree" :label="exts ? t('ui.pickFile') : t('ui.pickFolder')" :load="load" @activate="activate" />
</template>

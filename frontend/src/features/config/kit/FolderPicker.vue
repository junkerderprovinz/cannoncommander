<script setup lang="ts">
// The folder picker: the server's tree, folders only unless files with one of the given endings are wanted.
import { onMounted, reactive, ref } from 'vue';
import { call } from '@/api/client';
import { exists } from '@/api/admin';
import { G } from '@/core/glyphs';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { c, fail, setFoot } from './kit';
import Frow from './Frow.vue';

interface Node { name: string; path: string; dir: boolean; hasKids: boolean; label?: string; volume?: boolean }
const props = defineProps<{ win: WinEntry; start: string; exts: string[] | null; done: (p: string) => void }>();

const chosen = ref(props.start);
const open = reactive(new Set<string>(['/mnt']));
const kids = reactive(new Map<string, Node[]>());
const input = ref<HTMLInputElement | null>(null);
const ext = (n: string) => (n.includes('.') ? n.split('.').pop()!.toLowerCase() : '');
const join = (dir: string, name: string) => (dir === '/' ? '' : dir) + '/' + name;

async function load(path: string) {
  if (kids.has(path)) return;
  const q = '?' + new URLSearchParams({ path });
  const tree = (await call<{ items: { name: string; path: string; hasDirs: boolean; label?: string; total?: number }[] }>('/fs/tree' + q).catch(() => null))?.body?.items ?? [];
  const nodes: Node[] = tree.map((k) => ({ name: k.name, path: k.path, dir: true, hasKids: k.hasDirs, label: k.label, volume: !!k.total }));
  if (props.exts) {
    const files = (await call<{ items: { name: string; dir: boolean }[] }>('/fs/list' + q).catch(() => null))?.body?.items ?? [];
    for (const f of files) if (!f.dir && props.exts.includes(ext(f.name))) nodes.push({ name: f.name, path: join(path, f.name), dir: false, hasKids: false });
    // With files wanted, a folder can matter for what lies in it, so every folder may open.
    nodes.forEach((n) => { if (n.dir) n.hasKids = true; });
  }
  kids.set(path, nodes);
}

async function toggle(p: string) {
  if (open.has(p)) open.delete(p);
  else { open.add(p); await load(p); }
}

const glyphOf = (n: Node) => (!n.dir ? 'IconFile' : n.path === '/boot' ? 'IconFlash' : n.path === '/mnt/remotes' ? 'IconCloud' : n.volume ? 'IconZFS' : open.has(n.path) ? 'IconFolderOpen' : 'IconFolder');

function rows() {
  const out: { n: Node; depth: number }[] = [];
  const walk = (path: string, depth: number) => {
    for (const n of kids.get(path) ?? []) {
      out.push({ n, depth });
      if (n.dir && open.has(n.path)) walk(n.path, depth + 1);
    }
  };
  walk('/', 0);
  return out;
}

async function choose() {
  const p = chosen.value.trim();
  const kind = props.exts ? 'file' : 'dir';
  if (!(await exists(p, kind))) return fail(props.exts ? c('picker.needFile') : c('picker.noFolder'), input.value);
  props.done(p);
  props.win.close();
}

function pick(n: Node) { chosen.value = n.path; }
function dbl(n: Node) {
  if (props.exts && n.dir) return;
  chosen.value = n.path;
  choose();
}

onMounted(async () => {
  const parts = props.start.split('/').filter(Boolean);
  parts.forEach((_, k) => open.add('/' + parts.slice(0, k + 1).join('/')));
  await Promise.all(['/', ...open].map(load));
  setFoot(props.win, [cancelBtn(), { label: c('picker.choose'), g: 'IconCheck', primary: true, run: () => { choose(); return false; } }]);
});
</script>

<template>
  <Frow :label="exts ? c('picker.file') : c('picker.folder')"><input ref="input" v-model="chosen" class="field w mono" id="a-pick-path"></Frow>
  <div id="a-tree" class="well a-tree">
    <div v-for="{ n, depth } in rows()" :key="n.path" class="nav-row tree-row" :class="{ cur: chosen === n.path }" :style="{ paddingLeft: 0.3 + depth * 0.85 + 'rem' }" :data-pick="n.path" @click="pick(n)" @dblclick="dbl(n)">
      <button v-if="n.hasKids" class="tree-tog" :class="{ open: open.has(n.path) }" :aria-label="open.has(n.path) ? c('picker.collapse') : c('picker.expand')" @click.stop="toggle(n.path)"><G n="IconExpand" /></button>
      <span v-else class="tree-tog"></span>
      <G :n="glyphOf(n)" /><span>{{ n.label || n.name }}</span>
    </div>
  </div>
</template>

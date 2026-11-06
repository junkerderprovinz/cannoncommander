// Mehrfach umbenennen with patterns, Im Editor umbenennen with one name per line, and Nach Muster markieren. The
// server works out the new names in a dry run, so the preview and the renaming read the same metadata.
import { reactive, watch } from 'vue';
import { ApiError } from '@/api/client';
import { type RenameBatchResult, type RenameMask, type RenamePreset, type RenameRow, tools } from '@/api/tools';
import { action, run } from '@/core/actions';
import { MENUS } from '@/core/menus';
import { toast } from '@/core/toast';
import { go, view } from '@/core/views';
import { cancelBtn, win, type WinEntry } from '@/core/windows';
import { join } from '@/files/paths';
import { t, tl } from '@/i18n';
import { undoAct } from '@/features/undo/act';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';
import { plural, setPrimary, setStart } from '@/ui/kit';
import EditBody, { type EditState } from './EditBody.vue';
import PatternBody, { type PatternState } from './PatternBody.vue';
import PresetNameBody from './PresetNameBody.vue';
import RenameBody, { type RenameState } from './RenameBody.vue';
import './rename.css';

let lastMask: RenameMask | null = null;

// The rows of a rename as the toast and the marks need them, once the server has done it.
function renamed(rows: RenameRow[], result: RenameBatchResult, panel: number, dir: string) {
  const F = usePanels(), tab = F.T(panel), done = rows.filter((r) => r.new !== r.old);
  for (const r of done) if (tab.marked.delete(r.old)) tab.marked.add(r.new);
  useListing().changed(dir);
  toast(plural(done.length, 'rename.renamed'), 'ok', result.undo ? undoAct(result.undo) : undefined);
}
const failed = (e: unknown) => toast(e instanceof ApiError ? e.message : String(e), 'warn');

function multiRename() {
  const F = usePanels(), panel = F.active, dir = F.T().path, sel = F.sel();
  const paths = sel.map((k) => join(k.at || dir, k.name));
  const st: RenameState = reactive({
    o: { name: '[N]', ext: '[E]', start: 1, step: 1, find: '', repl: '', regex: false, case: 'keep', ...lastMask },
    rows: sel.map((k) => ({ path: join(k.at || dir, k.name), old: k.name, new: k.name })), bad: false, count: sel.length, own: [], preset: '',
  });
  tools.presets().then((items) => { st.own = items; }, () => {});

  async function apply() {
    try {
      const result = await tools.renameBatch({ paths, mask: { ...st.o } });
      lastMask = { ...st.o };
      renamed(result.rows, result, panel, dir);
    } catch (e) {
      failed(e);
    }
  }
  const w: WinEntry = win({
    title: t('rename.multi'), wide: true, cls: 'bk bk-mren', body: RenameBody, props: { st, savePreset: () => savePreset(st), dropPreset: () => dropPreset(st) },
    foot: [{ start: ' ' }, { label: t('rename.inEditor'), g: 'IconCode', run: () => { setTimeout(() => run('renameEditor')); } }, cancelBtn(),
      { label: t('rename.rename'), g: 'IconPencil', primary: true, disabled: true, run: () => { void apply(); } }],
  });

  // Every change asks the server for a dry run; an answer that arrives after a newer question is dropped.
  let asked = 0, timer: ReturnType<typeof setTimeout> | undefined;
  watch(() => ({ ...st.o }), (o) => {
    clearTimeout(timer);
    timer = setTimeout(async () => {
      const n = ++asked;
      try {
        const result = await tools.renameBatch({ paths, mask: o, dryRun: true });
        if (n !== asked) return;
        Object.assign(st, { rows: result.rows, bad: false });
      } catch (e) {
        if (n !== asked) return;
        if (e instanceof ApiError && e.status === 400) st.bad = true;
        else failed(e);
      }
      const errs = st.rows.filter((r) => r.error).length, changed = st.rows.filter((r) => r.new !== r.old).length;
      setStart(w, st.bad ? t('rename.badRegex') : errs ? plural(errs, 'rename.notFit') : '');
      setPrimary(w, { disabled: st.bad || !!errs || !changed });
    }, 80);
  }, { immediate: true, deep: true });
}

const own = (st: RenameState, name: string) => st.own.find((p) => p.name === name);
async function storePresets(st: RenameState, items: RenamePreset[]) {
  try {
    await tools.savePresets(items);
    st.own = items;
    return true;
  } catch {
    toast(t('rename.presetsFailed'), 'warn');
    return false;
  }
}

function savePreset(st: RenameState) {
  const ps = reactive({ name: st.o.name });
  const options = { name: st.o.name, ext: st.o.ext, find: st.o.find, repl: st.o.repl, regex: st.o.regex, case: st.o.case };
  win({
    title: t('rename.saveTitle'), body: PresetNameBody, props: { ps },
    foot: [cancelBtn(), { label: t('rename.save'), g: 'IconSave', primary: true, run: () => {
      const name = ps.name.trim();
      if (!name) return false;
      const items = own(st, name) ? st.own.map((p) => (p.name === name ? { name, options } : p)) : [...st.own, { name, options }];
      storePresets(st, items).then((ok) => {
        if (!ok) return;
        st.preset = 'own:' + name;
        toast(t('rename.saved', { name }));
      });
      return true;
    } }],
  });
}

function dropPreset(st: RenameState) {
  const name = st.preset.startsWith('own:') ? st.preset.slice(4) : '';
  if (!own(st, name)) return;
  storePresets(st, st.own.filter((p) => p.name !== name)).then((ok) => {
    if (!ok) return;
    st.preset = '';
    toast(t('rename.dropped', { name }));
  });
}

function renameEditor() {
  const F = usePanels(), panel = F.active, dir = F.T().path, sel = F.sel();
  const st: EditState = reactive({ dir, nodes: sel.map((k) => ({ name: k.name, at: k.at || dir })), text: sel.map((k) => k.name).join('\n'), rows: [], lines: sel.length });
  const w = win({
    title: t('rename.editor'), wide: true, cls: 'bk re-win', body: EditBody, props: { st },
    foot: [{ start: ' ' }, { label: t('rename.withMasks'), g: 'IconPencil', run: () => { setTimeout(() => run('multiRename')); } }, cancelBtn(),
      { label: t('rename.rename'), g: 'IconPencil', primary: true, disabled: true, run: () => {
        if (st.lines !== st.nodes.length || st.rows.some((r) => r.error)) return false;
        const rows = st.rows.map((r, i) => ({ path: join(st.nodes[i]!.at, r.old), old: r.old, new: r.new }));
        tools.renameBatch({ paths: rows.map((r) => r.path), names: rows.map((r) => r.new) })
          .then((result) => renamed(rows, result, panel, dir), failed);
        return true;
      } }],
  });
  watch(() => [st.rows, st.lines] as const, ([rows, lines]) => {
    const errs = rows.filter((r) => r.error).length;
    setStart(w, errs ? plural(errs, 'rename.notFit') : '');
    setPrimary(w, { disabled: lines !== st.nodes.length || !!errs || !rows.some((r) => r.new !== r.old) });
  }, { immediate: true });
}

let lastPattern = '*.jpg *.png', lastFolders = false, lastMode: 'add' | 'replace' = 'add';
function selectPattern() {
  if (view.id !== 'files') go('files');
  const F = usePanels();
  const st: PatternState = reactive({ pattern: lastPattern, folders: lastFolders, mode: lastMode, hits: [], total: 0 });
  const w = win({
    title: t('rename.pattern'), cls: 'bk', body: PatternBody, props: { st },
    foot: [{ start: ' ' }, cancelBtn(), { label: t('rename.mark'), g: 'IconSelectAll', primary: true, run: () => {
      if (!st.hits.length) return false;
      lastPattern = st.pattern; lastFolders = st.folders; lastMode = st.mode;
      const tab = F.T();
      if (st.mode === 'replace') tab.marked = new Set();
      st.hits.forEach((name) => tab.marked.add(name));
      return true;
    } }],
  });
  watch(() => [st.hits.length, st.total], ([n, total]) => {
    setStart(w, t('rename.matches', { n: n!, total: total! }));
    setPrimary(w, { disabled: !n });
  }, { immediate: true });
}

export default function setup() {
  action('multiRename', { label: tl('rename.multi'), g: 'IconPencil', key: 'Ctrl+m', needs: ['sel', 'writable'], run: multiRename });
  action('renameEditor', { label: tl('rename.editor'), g: 'IconCode', needs: ['sel', 'writable'], run: renameEditor });
  action('selectPattern', { label: tl('rename.pattern'), g: 'IconSelectAll', key: '+', run: selectPattern });
  const edit = MENUS.find(([id]) => id === 'edit')![1];
  if (!edit.includes('renameEditor')) edit.splice(edit.indexOf('multiRename') + 1, 0, 'renameEditor');
}

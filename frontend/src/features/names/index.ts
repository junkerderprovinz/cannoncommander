// Neuer Ordner (F7), Neue Datei (Umschalt+F4) and Umbenennen (F2). The server keeps each as an undo step, so Strg+Z
// takes it back without a toast asking first.
import { reactive } from 'vue';
import { type FsChanged, ops } from '@/api/ops';
import { action, actions, run } from '@/core/actions';
import { toast } from '@/core/toast';
import { cancelBtn, win } from '@/core/windows';
import { join } from '@/files/paths';
import { plural, t, tl } from '@/i18n';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';
import { useJobs } from '@/stores/jobs';
import { undoAct } from '@/features/undo/act';
import { freeName, namesIn, reveal } from '@/ui/tools';
import NameBody from './NameBody.vue';

// A duplicate sits next to its original under a free "Kopie" name, as on a Mac.
export function copyName(name: string, taken: Set<string>, word: string) {
  const dot = name.lastIndexOf('.'), end = dot > 0 ? name.slice(dot) : '';
  const stem = (dot > 0 ? name.slice(0, dot) : name).replace(new RegExp(` ${word}( \\d+)?$`), '');
  for (let n = 1; ; n++) {
    const c = `${stem} ${word}${n > 1 ? ' ' + n : ''}${end}`;
    if (!taken.has(c)) return c;
  }
}

function nameWindow(title: string, value: string, verb: string, glyph: string, apply: (name: string) => Promise<FsChanged | void>, old?: string, hint?: string, field?: string) {
  const F = usePanels(), listing = useListing(), folder = F.T().path, panel = F.active;
  const taken = (name: string) => !!listing.lists.get(folder)?.items.some((k) => k.name === name);
  const st = reactive({ name: value });
  win({
    title, body: NameBody, props: { st, hint, field },
    foot: [cancelBtn(), { label: verb, g: glyph, primary: true, run: () => {
      const name = st.name.trim();
      if (!name) return false;
      if (name !== old && taken(name)) { toast(t('names.taken'), 'warn'); return false; }
      if (name === old) return true;
      apply(name).then(async () => {
        await listing.load(folder).catch(() => null);
        const idx = F.entries(panel).findIndex((e) => e.name === name);
        if (idx >= 0 && F.T(panel).path === folder) F.T(panel).cursor = idx;
      }, (e) => toast(e.message, 'warn'));
      return true;
    } }],
  });
}

export default function setup() {
  const F = usePanels();
  const here = () => F.T().path;

  action('newFolder', { label: tl('names.newFolder'), g: 'IconFolderAdd', key: 'F7', needs: 'writable', run: () => {
    const folder = here();
    nameWindow(t('names.newFolder'), t('names.newFolder'), t('names.create'), 'IconFolderAdd', (name) => ops.mkdir(join(folder, name)));
  } });
  // The editor brings its own Neue Datei, which opens the file right away; this one only makes it.
  if (!actions.newFile) {
    action('newFile', { label: tl('names.newFile'), g: 'IconFileText', key: 'Umschalt+F4', needs: 'writable', run: () => {
      const folder = here();
      nameWindow(t('names.newFile'), t('names.newFileName'), t('names.create'), 'IconFileText', (name) => ops.touch(join(folder, name)));
    } });
  }
  action('folderWithSel', { label: tl('names.folderWithSel'), g: 'IconFolderAdd', key: 'Umschalt+F7', needs: ['sel', 'writable'], run: () => {
    const folder = here(), picked = F.sel().filter((k) => !k.at || k.at === folder);
    if (!picked.length) return toast(t('names.onlyHere'), 'warn');
    const hint = plural(picked.length, 'names.goIn');
    return nameWindow(t('names.folderWithSel'), freeName(namesIn(folder), t('names.newFolder')), t('names.create'), 'IconFolderAdd', async (name) => {
      const box = join(folder, name);
      F.T().marked = new Set();
      const job = await useJobs().start({ kind: 'move', params: { sources: picked.map((k) => join(folder, k.name)), dest: box, rule: 'skip', newFolder: true } });
      useJobs().whenDone(job, (done) => {
        if (done.state !== 'done') return toast(done.error || t('names.notMoved'), 'fault');
        void reveal(box);
        const what = plural(picked.length, 'names.objects');
        toast(t('names.folderMade', { name, what }), 'ok', done.undo ? undoAct(done.undo) : undefined);
      });
    }, undefined, hint, 'fws-name');
  } });

  action('duplicate', { label: tl('names.duplicate'), g: 'IconCopy', key: 'Strg+D', needs: ['sel', 'writable'], run: async () => {
    const folder = here(), sel = F.sel(), taken = namesIn(folder);
    const names = sel.map((k) => { const n = copyName(k.name, taken, t('names.copyWord')); taken.add(n); return n; });
    F.T().marked = new Set();
    try {
      const job = await useJobs().start({ kind: 'copy', params: { sources: sel.map((k) => join(folder, k.name)), dest: folder, names, rule: 'skip' } });
      useJobs().whenDone(job, (done) => {
        if (done.state !== 'done') return toast(done.error || t('names.notCopied'), 'fault');
        const one = names.length === 1 ? names[0]! : '';
        void reveal(join(folder, one || names[0]!));
        toast(one ? t('names.made', { name: one }) : plural(names.length, 'names.copies'), 'ok', done.undo ? undoAct(done.undo) : undefined);
      });
    } catch (e) { toast((e as Error).message, 'warn'); }
  } });

  action('rename', { label: tl('names.rename'), g: 'IconPencil', key: 'F2', needs: ['sel', 'writable'], run: () => {
    const sel = F.sel();
    if (sel.length > 1 && actions.multiRename) return run('multiRename');
    const k = sel[0]!, folder = k.at || here();
    return nameWindow(t('names.rename'), k.name, t('names.rename'), 'IconPencil', (name) => ops.rename(join(folder, k.name), name), k.name);
  } });
}

// Aufräumen: what a server collects over time, found by the server, sent to the trash or given a better name.
import { reactive, watch } from 'vue';
import { type CleanupScan, tools } from '@/api/tools';
import { action } from '@/core/actions';
import { MENUS } from '@/core/menus';
import { toast } from '@/core/toast';
import { view } from '@/core/views';
import { closeBtn, win } from '@/core/windows';
import { inArchive, parent, split } from '@/files/paths';
import { t, tl } from '@/i18n';
import { deleteItems } from '@/features/trash/trash';
import { undoAct } from '@/features/undo/act';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';
import { plural, setPrimary } from '@/ui/kit';
import { clean } from '@/ui/tools';
import CleanBody, { type CleanState } from './CleanBody.vue';
import { GROUPS, keyOf } from './groups';
import './cleanup.css';

function cleanupWindow() {
  const F = usePanels();
  const here = view.id === 'files' && !inArchive(F.T().path) ? F.T().path : '/mnt/user';
  const st: CleanState = reactive({ path: here, root: here, scan: null, missing: false, picked: new Set<string>() });
  const chosen = () => GROUPS.flatMap((gr) => (st.scan?.groups[gr.id] ?? []).filter((f) => st.picked.has(keyOf(gr.id, f))).map((f) => ({ ...f, gr })));

  let asked = 0;
  async function start() {
    const path = clean(st.path) || '/', n = ++asked;
    st.picked.clear();
    let got: CleanupScan | null = null;
    try { got = await tools.cleanupScan(path); } catch { /* the window says the folder is not there */ }
    if (n !== asked) return;
    Object.assign(st, { scan: got, missing: !got, root: path });
  }

  async function apply() {
    const all = chosen(), fixes = all.filter((f) => f.gr.act === 'fix'), dels = all.filter((f) => f.gr.act === 'delete');
    // A folder and something inside it may both be picked; the folder already takes the inner one along.
    const outer = dels.filter((f) => !dels.some((o) => o !== f && f.path.startsWith(o.path + '/')));
    if (fixes.length) {
      const req = { paths: fixes.map((f) => f.path), names: fixes.map((f) => f.to!) };
      try {
        const dry = await tools.renameBatch({ ...req, dryRun: true });
        const ok = dry.rows.map((r, i) => [r, i] as const).filter(([r]) => !r.error).map(([, i]) => i);
        if (ok.length) {
          const done = await tools.renameBatch({ paths: ok.map((i) => req.paths[i]!), names: ok.map((i) => req.names[i]!) });
          toast(plural(ok.length, 'cleanup.fixed'), 'ok', done.undo ? undoAct(done.undo) : undefined);
        }
        if (ok.length < fixes.length) toast(plural(fixes.length - ok.length, 'cleanup.kept'), 'warn');
        new Set(req.paths.map(parent)).forEach((d) => useListing().changed(d));
      } catch (e) {
        toast((e as Error).message, 'warn');
      }
    }
    if (outer.length) deleteItems(outer.map((f) => ({ name: split(f.path).pop()!, at: parent(f.path), dir: f.gr.id === 'emptyDir', kind: 'file', size: f.size || 0, time: 0, mode: '', owner: '', group: '' })), st.root, { after: start });
    else void start();
  }
  const w = win({
    title: t('cleanup.title'), wide: true, cls: 'bk bk-clean', body: CleanBody, props: { st },
    foot: [{ start: t('cleanup.toTrash') }, closeBtn(), { label: t('cleanup.title'), g: 'IconPrune', primary: true, disabled: true, run: () => { void apply(); return false; } }],
  });
  watch(() => [st.picked.size, st.scan], () => {
    const all = chosen(), del = all.filter((f) => f.gr.act === 'delete').length, fix = all.length - del;
    setPrimary(w, {
      label: all.length ? [del && t('cleanup.delete', { n: del }), fix && t('cleanup.fix', { n: fix })].filter(Boolean).join(', ') : t('cleanup.title'),
      disabled: !all.length,
    });
  });
  let typing: ReturnType<typeof setTimeout> | undefined;
  watch(() => st.path, () => { clearTimeout(typing); typing = setTimeout(start, 400); });
  void start();
}

export default function setup() {
  action('cleanup', { label: tl('cleanup.title'), g: 'IconPrune', run: cleanupWindow });
  const menu = MENUS.find(([id]) => id === 'tools')![1];
  if (!menu.includes('cleanup')) menu.splice(menu.indexOf('duplicates') + 1, 0, 'cleanup');
}

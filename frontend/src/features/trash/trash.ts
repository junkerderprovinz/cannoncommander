// Deleting into the trash or for good, and restoring or purging what waits in it. Every one of these runs as a job;
// its toast comes once the job is through.
import { reactive, watch } from 'vue';
import { type Job, ops, type TrashItem } from '@/api/ops';
import { fmtSize } from '@/core/format';
import { host } from '@/core/host';
import { prefs } from '@/core/prefs';
import { toast } from '@/core/toast';
import { cancelBtn, win } from '@/core/windows';
import type { Entry } from '@/files/entries';
import { t } from '@/i18n';
import { confirmWin } from '@/features/config/shared';
import { objects } from '@/features/jobs/label';
import { pathOf } from '@/features/transfer/paths';
import { undoAct } from '@/features/undo/act';
import { useJobs } from '@/stores/jobs';
import { usePanels } from '@/stores/panels';
import { useTrash } from '@/stores/trash';
import DeleteBody from './DeleteBody.vue';
import PurgeBody from './PurgeBody.vue';

const failed = (j: Job) => toast(t('trash.failed', { error: j.error || j.state }), 'fault');

function whenThrough(job: Job, ok: (j: Job) => void, fail: (j: Job) => unknown = failed) {
  useJobs().whenDone(job, (j) => (j.state === 'done' ? ok(j) : j.state === 'failed' ? fail(j) : undefined));
}

// The trash of Windows or of the Linux desktop, in the desktop app.
export const binName = () => t(host.os === 'windows' ? 'trash.binWindows' : 'trash.binSystem');

// A drive without a trash, such as a network drive, gets the question the Explorer asks: delete for good instead? A
// refusal for lack of rights is left to the offer to go on as administrator.
async function trashFailed(j: Job, after?: () => void) {
  const items = (await ops.job(j.id).catch(() => null))?.items ?? [];
  const homeless = items.filter((i) => i.errorCode === 'noSystemTrash');
  if (!homeless.length) {
    if (!host.desktop || !items.some((i) => i.errorCode === 'denied')) failed(j);
    return;
  }
  const what = homeless.length === 1 ? homeless[0]!.name || homeless[0]!.path : objects(homeless.length);
  confirmWin({
    title: t('trash.noBinTitle'), text: [t('trash.noBinText', { what })], verb: t('trash.forever'), glyph: 'IconTrash',
    run: () => { void removeForGood(homeless.map((i) => i.path), what, after); },
  });
}

async function removeForGood(paths: string[], what: string, after?: () => void) {
  try {
    whenThrough(await useJobs().start({ kind: 'delete', params: { paths } }), () => {
      after?.();
      toast(t('trash.deleted', { what }), 'ok');
    });
  } catch (e) {
    toast((e as Error).message, 'warn');
  }
}

// Deletes items of the folder at path, or of their own folder for hits from deeper down; after() redraws a window
// that listed them.
export function deleteItems(sel: Entry[], path: string, { forever = false, after }: { forever?: boolean; after?: () => void } = {}) {
  const { days, system } = useTrash();
  const doIt = async (toTrash: boolean) => {
    const what = sel.length === 1 ? sel[0]!.name : objects(sel.length);
    try {
      const job = await useJobs().start({ kind: toTrash ? 'trash' : 'delete', params: { paths: sel.map((k) => pathOf(k, path)) } });
      usePanels().T().marked = new Set();
      whenThrough(job, (j) => {
        after?.();
        toast(toTrash ? t('trash.inTrash', { what }) : t('trash.deleted', { what }), 'ok', toTrash && j.undo ? undoAct(j.undo) : undefined);
      }, (j) => trashFailed(j, after));
    } catch (e) {
      toast((e as Error).message, 'warn');
    }
  };
  const trash = !forever && prefs.trash !== false;
  if (!prefs.confirmDelete && !forever) {
    void doIt(trash);
    return;
  }
  const st = reactive({ trash });
  const primary = () => ({ label: st.trash ? t('trash.toTrash') : t('trash.forever'), g: 'IconTrash', primary: true, run: () => { void doIt(st.trash); } });
  const w = win({
    title: t('trash.delete'), cls: st.trash ? 'xwin' : 'xwin forever', body: DeleteBody, props: { items: sel, days, st, bin: system ? binName() : undefined }, foot: [cancelBtn(), primary()],
    onClose: () => stop(),
  });
  const stop = watch(() => st.trash, () => {
    w.cls = st.trash ? 'xwin' : 'xwin forever';
    w.foot = w.foot.map((f) => (f.primary ? primary() : f));
  });
}

const sizeOf = (list: TrashItem[]) => list.reduce((a, e) => a + (e.size || 0), 0);

export async function restore(list: TrashItem[], done?: () => void) {
  if (!list.length) return;
  const what = list.length === 1 ? list[0]!.name : objects(list.length);
  const from = [...new Set(list.map((e) => e.from))];
  try {
    const job = await useTrash().restore(list);
    done?.();
    whenThrough(job, () => toast(from.length === 1 ? t('trash.restoredTo', { what, from: from[0]! }) : t('trash.restored', { what }), 'ok',
      from.length === 1 ? { label: t('trash.go'), run: () => { usePanels().go(from[0]!); } } : undefined));
  } catch (e) {
    toast((e as Error).message, 'warn');
  }
}

export function purge(list: TrashItem[], all = false, done?: () => void) {
  if (!list.length) return;
  const size = sizeOf(list), what = objects(list.length);
  win({
    title: all ? t('trash.empty') : t('trash.purge'), body: PurgeBody, props: { list, what, size },
    foot: [cancelBtn(), { label: all ? t('trash.empty') : t('trash.purge'), g: all ? 'IconEmpty' : 'IconTrash', primary: true, run: () => {
      const trash = useTrash();
      (all ? trash.empty() : trash.purge(list)).then((job) => {
        done?.();
        whenThrough(job, () => toast(t('trash.purged', { what, size: fmtSize(size) || '0 B' })));
      }, (e) => toast(e.message, 'warn'));
    } }],
  });
}

// Ordner bündeln for admins: the server plans, the window shows the plan, and the move runs as a gather or scatter job.
import { reactive, watch } from 'vue';
import { type GatherPlan, type Job, ops } from '@/api/ops';
import { actionsWhere, run } from '@/core/actions';
import { fmtSize } from '@/core/format';
import { MENUS } from '@/core/menus';
import { toast } from '@/core/toast';
import { view } from '@/core/views';
import { win } from '@/core/windows';
import { join, split } from '@/files/paths';
import { plural, t, tl } from '@/i18n';
import { diskName, jobError, jobLabel } from '@/features/jobs/label';
import { useUnraid } from '@/features/unraid/state';
import { useJobs } from '@/stores/jobs';
import { usePanels } from '@/stores/panels';
import { useUser } from '@/stores/user';
import GatherBody, { type GatherState } from './GatherBody.vue';
import './gather.css';

const inShare = (p: string) => /^\/mnt\/user\/[^/]+/.test(p);
const files = (n: number) => plural(n, 'gather.files');

function planText(st: GatherState, plan: GatherPlan) {
  const disk = diskName(st.disk);
  const cache = plan.cached ? plural(plan.cached, 'gather.cached') : '';
  if (st.mode === 'gather') {
    const from = plan.devices.filter((d) => d.device !== st.disk && d.after < d.now).map((d) => diskName(d.device)).join(t('gather.and'));
    return (plan.short ? t('gather.short', { disk, size: fmtSize(plan.short) }) : !plan.moves ? t('gather.already', { disk })
      : t('gather.moves', { size: fmtSize(plan.move), files: files(plan.moves), from, disk })) + cache;
  }
  if (!st.disk) return t('gather.nothing') + cache;
  if (!plan.moves) return t('gather.noRoom', { size: fmtSize(plan.short) }) + cache;
  const to = plan.devices.filter((d) => d.after > d.now).map((d) => t('gather.toDisk', { disk: diskName(d.device), size: fmtSize(d.after - d.now) })).join(t('gather.and'));
  return t('gather.spreads', { size: fmtSize(plan.move), disk, to }) + (plan.short ? t('gather.stays', { size: fmtSize(plan.short) }) : '') + cache;
}

function gatherWindow() {
  const F = usePanels(), sel = F.sel();
  const here = view.id === 'files' ? F.T().path : '/mnt/user';
  const start = sel.length === 1 && sel[0]!.dir && !sel[0]!.link ? join(sel[0]!.at || here, sel[0]!.name) : here;
  const st: GatherState = reactive({ path: inShare(start) ? start : '/mnt/user/filme', mode: 'gather', disk: '', plan: null, valid: false, text: '' });
  const primary = () => {
    const ok = !!st.plan?.moves && (st.mode === 'scatter' || !st.plan.short) && st.valid;
    const label = !st.valid ? t('gather.start') : st.mode === 'gather' ? t('gather.fetch', { disk: diskName(st.disk) }) : st.disk ? t('gather.empty', { disk: diskName(st.disk) }) : t('gather.emptyPlain');
    return { label, g: 'IconMerge', primary: true, disabled: !ok, run: () => { void go(); } };
  };
  const w = win({ title: t('gather.title'), wide: true, cls: 'bk u-win u-gather', body: GatherBody, props: { st }, foot: [{ start: t('gather.background') }, primary()] });

  // A changed folder or task asks for a new suggestion; a disk picked by hand is asked about as it is.
  let asked = 0;
  async function plan(keepDisk: boolean) {
    const path = st.path.trim().replace(/(.)\/$/, '$1'), n = ++asked;
    st.valid = inShare(path);
    if (!st.valid) { st.plan = null; st.text = ''; w.foot = w.foot.map((f) => (f.primary ? primary() : f)); return; }
    const got = await ops.gatherPlan(path, st.mode, keepDisk ? st.disk || undefined : undefined).catch((e) => { toast(e.message, 'warn'); return null; });
    if (n !== asked) return;
    st.plan = got;
    st.valid = !!got;
    if (got) {
      st.disk = got.disk ?? '';
      st.text = planText(st, got);
    }
    w.foot = w.foot.map((f) => (f.primary ? primary() : f));
  }
  watch(() => [st.path, st.mode], () => { void plan(false); }, { immediate: true });
  watch(() => st.disk, (d, was) => { if (was !== undefined && st.plan && d !== st.plan.disk) void plan(true); });

  async function go() {
    if (!st.plan?.moves) return;
    const path = st.path.trim().replace(/(.)\/$/, '$1'), name = split(path).pop()!, mode = st.mode, disk = st.disk;
    useUnraid().wake(st.plan.devices.filter((d) => d.device === disk || d.after !== d.now).map((d) => d.device));
    try {
      const job: Job = await useJobs().start({ kind: mode, params: { path, disk } });
      toast(t('gather.running', { label: jobLabel(job) }), 'ok', { label: t('gather.toJobs'), run: () => run('jobs') });
      useJobs().whenDone(job, (j) => {
        if (j.state === 'done') toast(t(mode === 'gather' ? 'gather.gathered' : 'gather.scattered', { name, disk: diskName(disk) }));
        else if (j.state === 'failed') toast(t('gather.failed', { label: jobLabel(j), error: jobError(j) }), 'fault');
      });
    } catch (e) {
      toast((e as Error).message, 'warn');
    }
  }
}

export default function setup() {
  const user = useUser(), U = useUnraid();
  actionsWhere(() => U.available, { gather: { label: tl('gather.title'), g: 'IconMerge', when: () => user.admin(), run: gatherWindow } });
  const tools = MENUS.find(([id]) => id === 'tools')![1];
  if (!tools.includes('gather')) tools.splice(tools.indexOf('diskUsage') + 1, 0, 'gather');
}

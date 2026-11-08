// Going on with administrator rights in the desktop app. A copy, move, delete, restore or undo that the system refused
// for lack of rights gets the question the Explorer asks, and Fortsetzen runs the refused items again through a helper
// that the system starts after its own prompt. A job refused as a whole, such as a move into a new folder that could
// not be made, goes on with all its open items.
import { type Job, type JobItem, ops } from '@/api/ops';
import { host } from '@/core/host';
import { toast } from '@/core/toast';
import { confirmWin } from '@/features/config/shared';
import { objects } from '@/features/jobs/label';
import { t } from '@/i18n';
import { useJobs } from '@/stores/jobs';

const KINDS = ['copy', 'move', 'delete', 'trash', 'restore', 'undo'];

export default function setup() {
  useJobs().onEnded((j) => {
    if (host.desktop && j.state === 'failed' && KINDS.includes(j.kind)) void offer(j);
  });
}

const nameOf = (i: JobItem) => i.name || i.path.split('/').pop()!;

async function offer(j: Job) {
  const items = (await ops.job(j.id).catch(() => null))?.items ?? [];
  const failed = (code: string) => items.filter((i) => i.state === 'failed' && i.errorCode === code);
  const said = failed('elevationDeclined');
  if (said.length) {
    toast(t('elevate.declined', { what: said.length === 1 ? nameOf(said[0]!) : objects(said.length) }), 'warn');
    return;
  }
  const open = items.filter((i) => i.state === 'pending' || i.state === 'running');
  const denied = failed('denied').length ? failed('denied') : j.errorCode === 'denied' ? open : [];
  if (!denied.length) return;
  const what = denied.length === 1 ? nameOf(denied[0]!) : objects(denied.length);
  confirmWin({
    title: t('elevate.title'),
    text: [t(`elevate.text.${j.kind}`, { what }), t(host.os === 'windows' ? 'elevate.howWindows' : 'elevate.howLinux')],
    verb: t('elevate.go'),
    glyph: 'IconShieldOn',
    run: async () => {
      try {
        useJobs().upsert(await ops.jobStep(j.id, 'elevate'));
      } catch (e) {
        toast((e as Error).message, 'warn');
      }
    },
  });
}

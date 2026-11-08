// Going on with administrator rights in the desktop app. A copy, move or delete that the system refused for lack of
// rights gets the question the Explorer asks, and Fortsetzen runs the refused items again through a helper that the
// system starts after its own prompt.
import { type Job, ops } from '@/api/ops';
import { host } from '@/core/host';
import { toast } from '@/core/toast';
import { confirmWin } from '@/features/config/shared';
import { objects } from '@/features/jobs/label';
import { t } from '@/i18n';
import { useJobs } from '@/stores/jobs';

const KINDS = ['copy', 'move', 'delete', 'trash'];

export default function setup() {
  useJobs().onEnded((j) => {
    if (host.desktop && j.state === 'failed' && KINDS.includes(j.kind)) void offer(j);
  });
}

async function offer(j: Job) {
  const items = (await ops.job(j.id).catch(() => null))?.items ?? [];
  const denied = items.filter((i) => i.state === 'failed' && i.errorCode === 'denied');
  if (!denied.length) return;
  const what = denied.length === 1 ? denied[0]!.name || denied[0]!.path.split('/').pop()! : objects(denied.length);
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

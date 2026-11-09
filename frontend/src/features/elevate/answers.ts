// A job the desktop app offers to run as administrator gets that offer, or is told the rights were not given, and no
// failure toast besides.
import { type Job, type JobItem, ops } from '@/api/ops';
import { host } from '@/core/host';

export const ELEVATABLE = ['copy', 'move', 'delete', 'trash', 'restore', 'undo'];

export const answeredBy = (j: Job, items: JobItem[]) => host.desktop && ELEVATABLE.includes(j.kind)
  && (j.errorCode === 'denied' || items.some((i) => i.state === 'failed' && (i.errorCode === 'denied' || i.errorCode === 'elevationDeclined')));

export async function elevationAnswers(j: Job) {
  if (!host.desktop || !ELEVATABLE.includes(j.kind)) return false;
  return answeredBy(j, (await ops.job(j.id).catch(() => null))?.items ?? []);
}

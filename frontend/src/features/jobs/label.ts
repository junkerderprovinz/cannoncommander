// How a job reads in the list, in toasts and in notices: what it does to what, and where it stands.
import type { Job } from '@/api/ops';
import { fmtLeft, fmtSize, MB } from '@/core/format';
import { prefs } from '@/core/prefs';
import { split } from '@/files/paths';
import { locale, plural, serverText, t } from '@/i18n';
import { useListing } from '@/stores/listing';

export const placeName = (path?: string) => (path ? useListing().placeLabel(path) || split(path).pop() || t('jobs.server') : t('jobs.server'));
// A disk goes by its label, such as Disk 2, also where the tree has not been read yet.
export const diskName = (dev?: string) => (dev ? useListing().placeLabel('/mnt/' + dev) || dev.replace(/^disk(\d+)$/, 'Disk $1').replace(/^cache$/, 'Cache') : '');

export const objects = (n: number) => plural(n, 'jobs.objects');
// Why a job failed, in the interface's language where the server named it.
export const jobError = (j: Job) => serverText(j.errorCode, j.errorParams) ?? j.error ?? '';

type Labelled = Pick<Job, 'kind' | 'name' | 'count' | 'dest'>;
// Features name the kinds of job they start themselves; what is the one name or the count the default label shows.
export const kindLabels: Record<string, (j: Labelled, what: string) => string> = {};

export function jobLabel(j: Labelled) {
  const what = j.count === 1 ? j.name : j.kind === 'upload' ? plural(j.count, 'jobs.files') : objects(j.count);
  if (kindLabels[j.kind]) return kindLabels[j.kind]!(j, what);
  const dev = j.dest?.replace(/^\/mnt\//, '');
  const to = j.kind === 'gather' || j.kind === 'scatter' ? diskName(dev) : placeName(j.dest);
  const kind = ['copy', 'move', 'delete', 'trash', 'restore', 'emptyTrash', 'undo', 'upload', 'gather', 'scatter', 'fixPerms'].includes(j.kind) ? j.kind : 'other';
  return t(`jobs.kind.${kind}`, { what, to, kind: j.kind });
}

export const pctOf = (j: Job) => (j.state === 'done' ? 100 : j.totalBytes ? (j.doneBytes / j.totalBytes) * 100 : j.totalFiles ? (j.doneFiles / j.totalFiles) * 100 : 0);

export function factsOf(j: Job) {
  const facts = [j.totalBytes ? t('jobs.of', { done: fmtSize(j.doneBytes) || '0 B', total: fmtSize(j.totalBytes) }) : ''];
  if (j.state === 'running' && j.rate) facts.push(t('jobs.perSecond', { size: fmtSize(j.rate) }), j.left != null ? fmtLeft(j.left) : '');
  else if (j.state === 'queued') facts.push(j.reason === 'parity' ? t('jobs.parity') : t('jobs.place', { n: j.place ?? 1 }));
  else if (j.state === 'asking') facts.push(t('jobs.asking'));
  else if (j.state === 'failed' && j.error) facts.push(t('jobs.failedWith', { error: jobError(j) }));
  else facts.push(t(`jobs.state.${j.state}`));
  const cap = j.cap ? Math.round(j.cap / MB) : Number(prefs.speedCap) || 0;
  if (cap && j.state === 'running') facts.push(t('jobs.capped', { n: cap }));
  return facts.filter(Boolean).join(' · ');
}

export function took(secs: number) {
  if (secs < 60) return t('jobs.took.s', { n: Math.round(secs) });
  if (secs < 3600) return t('jobs.took.min', { n: Math.round(secs / 60) });
  return t('jobs.took.h', { n: (secs / 3600).toLocaleString(locale(), { minimumFractionDigits: 1, maximumFractionDigits: 1 }) });
}

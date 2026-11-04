// How a share keeps its files, in the words of the pill in the panel foot: primary and secondary storage, which
// way the mover goes and when.
import type { UnraidShare, UnraidStorage } from '@/api/unraid';
import { fmtSize } from '@/core/format';
import { t } from '@/i18n';
import { diskName } from '@/features/jobs/label';
import { names } from './state';

const NUM = /^\d+$/;

// A crontab line as people say it: a time of day, every few hours, or the next run for anything rarer.
export function moverWhen(cron?: string) {
  const [m = '', h = '', ...rest] = (cron || '').trim().split(/\s+/);
  if (rest.length !== 3 || rest.some((x) => x !== '*') || !NUM.test(m)) return t('unraid.when.next');
  if (NUM.test(h)) return t('unraid.when.at', { time: `${+h}:${m.padStart(2, '0')}` });
  const every = h === '*' ? 1 : Number(h.match(/^\*\/(\d+)$/)?.[1]);
  if (every === 1) return t('unraid.when.hourly');
  return every > 1 ? t('unraid.when.hours', { n: every }) : t('unraid.when.next');
}

// The pool as the object of a sentence: "auf dem Cache", but "auf fastcache".
export const poolPhrase = (pool = 'cache') => (pool === 'cache' ? t('unraid.theCache') : diskName(pool));
export const storeName = (s: UnraidStorage) => (s.type === 'array' ? t('unraid.array') : s.name === 'cache' ? t('unraid.cache') : diskName(s.name));

// The pool the mover still has to empty, for a share that writes to a pool first and keeps its files on the array.
export const moverPool = (sh?: UnraidShare) => (sh?.mover === 'toSecondary' && sh.primary.type === 'pool' && sh.secondary?.type === 'array' ? sh.primary.name || 'cache' : '');

export interface Waiting { files: number; size: number }
export interface Badge { label: string; glyph: string; tip: string }

export function shareBadge(sh: UnraidShare, schedule?: string, waiting?: Waiting): Badge {
  const sec = sh.secondary && (sh.secondary.type === 'pool' || sh.disks.length) ? sh.secondary : undefined;
  const first = storeName(sh.primary), second = sec ? storeName(sec) : '';
  const label = !second ? t('unraid.only', { store: first }) : sh.mover === 'toPrimary' ? `${second} → ${first}` : `${first} → ${second}`;
  const pool = poolPhrase(sh.primary.name), disks = sec?.type === 'pool' ? poolPhrase(sec.name) : names(sh.disks);
  let tip = t('unraid.shareTip', { share: sh.name, first, second: second ? t('unraid.secondary', { second }) : t('unraid.noSecondary') });
  if (!second) tip += sh.primary.type === 'pool' ? t('unraid.stays', { pool }) : t('unraid.straight', { disks });
  else if (sh.mover === 'toPrimary') tip += t('unraid.toPrimary', { pool, disks });
  else tip += t('unraid.toSecondary', { pool, disks, when: moverWhen(schedule) });
  if (second && sh.mover === 'toSecondary' && waiting?.files) {
    const size = fmtSize(waiting.size) || '0 B';
    tip += waiting.files === 1 ? t('unraid.waitingOne', { size, pool }) : t('unraid.waitingMany', { n: waiting.files, size, pool });
  }
  return { label, glyph: sh.primary.type === 'pool' ? 'IconBolt' : 'IconDatabase', tip };
}

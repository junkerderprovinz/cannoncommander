// Drives of the computer the desktop app runs on, named the way its own file manager names them: the label the drive
// was given, or what kind of drive it is, and on Windows its letter, as in USB-Laufwerk (E:).
import type { DriveKind } from '@/api/types';
import { fmtSize } from '@/core/format';
import { t } from '@/i18n';

export interface DriveLike { path?: string; label?: string; kind?: DriveKind; remote?: string; size?: number }

const GLYPHS: Record<DriveKind, string> = { fixed: 'IconDatabase', removable: 'IconFlash', optical: 'IconDisc', network: 'IconNetwork', virtual: 'IconDisc' };
export const driveGlyph = (kind: DriveKind) => GLYPHS[kind];

export function driveName(d: DriveLike): string {
  const letter = /^\/([A-Za-z])$/.exec(d.path ?? '')?.[1];
  const share = /^\\\\([^\\]+)\\(.+)$/.exec(d.remote ?? '');
  const kind = d.kind === 'virtual' ? 'fixed' : d.kind ?? 'fixed';
  const word = d.label
    || (share ? `${share[2]} (\\\\${share[1]})` : '')
    || (d.kind === 'network' ? d.remote : '')
    || (!letter && d.size ? t('drives.volume', { size: fmtSize(d.size) }) : '')
    || t(`drives.${kind}`);
  return letter ? `${word} (${letter.toUpperCase()}:)` : word;
}

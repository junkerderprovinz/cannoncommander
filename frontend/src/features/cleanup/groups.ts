// The groups Aufräumen reports, and whether a ticked find goes to the trash or gets the name the server suggests.
import type { CleanupFind, CleanupGroup } from '@/api/tools';

export const GROUPS: { id: CleanupGroup; g: string; act: 'delete' | 'fix' }[] = [
  { id: 'emptyDir', g: 'IconFolder', act: 'delete' },
  { id: 'emptyFile', g: 'IconFile', act: 'delete' },
  { id: 'temp', g: 'IconHourglass', act: 'delete' },
  { id: 'link', g: 'IconUnlink', act: 'delete' },
  { id: 'ext', g: 'IconWarning', act: 'fix' },
  { id: 'name', g: 'IconPencil', act: 'fix' },
];
export const keyOf = (group: string, f: CleanupFind) => `${group}|${f.path}`;

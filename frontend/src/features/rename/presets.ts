// The patterns that come with the app. Your own ones live on the server under me/rename-presets.
import type { RenameMask, RenamePreset } from '@/api/tools';

export const CASES = ['keep', 'lower', 'upper', 'title'] as const;
const pattern = (name: string, more: Partial<RenameMask> = {}): RenamePreset['options'] => ({ name, ext: '[E]', find: '', repl: '', regex: false, case: 'keep', ...more });
export const BUILT_IN: { key: string; options: RenamePreset['options'] }[] = [
  { key: 'photos', options: pattern('[A]_[C3]') },
  { key: 'music', options: pattern('[I] - [T]') },
  { key: 'series', options: pattern('S[S2]E[F2]') },
  { key: 'resolution', options: pattern('[N] [R]') },
];

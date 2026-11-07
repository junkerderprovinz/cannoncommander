// What Explorer's address bar does with cmd or powershell, the path field does with these: it opens a terminal here.
import { join } from '@/files/paths';

export const TRIGGERS = ['cmd', 'sh', 'bash', 'terminal', 'powershell', 'pwsh', 'wt'];

// A folder here that bears the word, in any case, is what the field opens instead, so no name ever becomes unreachable.
export function pathCommand(typed: string, here: string, items: { name: string; dir: boolean }[]): { terminal: true } | { folder: string } | null {
  const word = typed.trim();
  if (!TRIGGERS.includes(word.toLowerCase())) return null;
  const dirs = items.filter((k) => k.dir && k.name.toLowerCase() === word.toLowerCase());
  const dir = dirs.find((k) => k.name === word) ?? dirs[0];
  return dir ? { folder: join(here, dir.name) } : { terminal: true };
}

// The programs the server names for a file, and opening the file with one of them.
import { call, json } from '@/api/client';
import { toast } from '@/core/toast';

export interface App { id: string; name: string; icon?: string; default?: boolean }
export interface Apps { apps: App[]; dialog: boolean }

export const warn = (e: unknown) => toast(e instanceof Error ? e.message : String(e), 'warn');

// every says the program came from the list of all programs, which Linux shows in place of a dialog.
export async function openWith(path: string, app: App, every = false) {
  try {
    await call('/openwith', json('POST', { path, app: app.id, every }));
  } catch (e) {
    warn(e);
  }
}

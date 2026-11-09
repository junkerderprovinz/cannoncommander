// The program F4 opens text files in, in the desktop app: one the computer offers for text files, picked from the
// list Öffnen mit shows, or a program file picked in the system's dialog. None means the built-in editor.
import { ApiError, call, json, q } from '@/api/client';
import { windowsPaths } from '@/core/host';
import { prefs, setPref } from '@/core/prefs';
import { toast } from '@/core/toast';
import type { App, Apps } from '@/features/openwith/apps';
import { typedPath } from '@/files/paths';
import { t } from '@/i18n';
import { useUser } from '@/stores/user';

// app is a program the computer offers, by its id, for text files or with every for all programs where Linux has no
// dialog. file is a program file, kept as the computer writes its path.
export interface EditorProgram { kind: 'app' | 'file'; id: string; name: string; icon?: string; every?: boolean }

// The type the programs are listed for. Every editor worth picking opens a .txt.
export const TEXT_TYPE = 'txt';

export const editorProgram = () => (prefs.editorProgram as EditorProgram | null | undefined) ?? null;
export const setEditorProgram = (p: EditorProgram | null) => setPref('editorProgram', p);

export async function textPrograms(every = false): Promise<Apps> {
  return (await call<Apps>('/openwith' + q({ type: TEXT_TYPE, ...(every ? { every: 'true' } : {}) }))).body ?? { apps: [], dialog: false };
}

// The system's own dialog, kept to programs. Closing it picks nothing.
export async function pickProgram(): Promise<EditorProgram | null> {
  const got = await call<App>('/openwith/pick', json('POST', { title: t('editor.program.pickTitle'), programs: t('editor.program.programs') }));
  const a = got.body;
  return a ? { kind: 'file', id: a.id, name: a.name, icon: a.icon } : null;
}

export async function openInProgram(p: EditorProgram, path: string) {
  try {
    if (p.kind === 'file') {
      const program = typedPath(p.id, { windows: windowsPaths(), home: useUser().me.home || '/' });
      await call('/openwith/program', json('POST', { path, program }));
    } else {
      await call('/openwith', json('POST', { path, app: p.id, ...(p.every ? { every: true } : { type: TEXT_TYPE }) }));
    }
  } catch (e) {
    const code = e instanceof ApiError ? e.problem?.code : undefined;
    // A program that is gone from the computer is named, with where to pick another.
    toast(code === 'unknownApp' || code === 'notAProgram' ? t('editor.program.gone', { name: p.name }) : e instanceof Error ? e.message : String(e), 'warn');
  }
}

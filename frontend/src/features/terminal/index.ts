// Terminal hier öffnen: a shell on the server in the folder of the panel, for admins once the switch in the settings
// is on. Typing cmd, bash or powershell into the path field opens one too, as in Explorer. The desktop app opens the
// computer's own terminal instead, in a window of its own.
import './terminal.css';
import { defineAsyncComponent, watch, watchEffect } from 'vue';
import { call, json, q } from '@/api/client';
import { action, actions } from '@/core/actions';
import { addTo } from '@/core/extend';
import { toast } from '@/core/toast';
import { go } from '@/core/views';
import { win } from '@/core/windows';
import { t, tl } from '@/i18n';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';
import { useUser } from '@/stores/user';
import { onDesktop, refreshTerminal, terminalAllowed, terminalOn } from './state';
import { pathCommand } from './words';

// xterm comes in a chunk of its own, fetched when the first terminal opens.
const TerminalBody = defineAsyncComponent(() => import('./TerminalBody.vue'));

function openWindow(path: string) {
  win({
    title: t('terminal.title'), full: true, cls: 'bk bk-term', body: TerminalBody, props: { path },
    foot: [{ start: path }, { label: t('terminal.another'), g: 'IconAdd', run: () => { void askTerminal(path); return false; } }],
  });
}

// The word typed into the path field picks the program in the desktop app, where the window is the computer's own.
async function openNative(path: string, kind: string) {
  try {
    await call('/terminal/native', json('POST', { path, kind }));
  } catch (e) {
    toast(e instanceof Error ? e.message : String(e), 'warn');
  }
}

// The server decides in the end. Asking it first gives a refusal in words, which a refused WebSocket never carries.
export async function askTerminal(path: string, kind = 'terminal') {
  if (!useUser().admin()) return toast(t('terminal.adminsOnly'), 'warn');
  if (!terminalOn.value) await refreshTerminal();
  if (import.meta.env.DESKTOP && onDesktop.value) return openNative(path, kind);
  if (!terminalOn.value) return toast(t('terminal.off'), 'warn', { label: t('terminal.toSettings'), run: () => go('settings', 'integrations') });
  try {
    await call('/terminal' + q({ path }));
  } catch (e) {
    return toast(e instanceof Error ? e.message : String(e), 'warn');
  }
  openWindow(path);
}

export default function setup() {
  const F = usePanels(), listing = useListing(), user = useUser();
  watch(() => user.admin(), (admin) => { if (admin) void refreshTerminal(); }, { immediate: true });
  // Only those who may open one see the command, in the menus and in the command search alike.
  watchEffect(() => {
    if (terminalAllowed()) action('terminal', { label: tl('terminal.open'), g: 'IconCode', run: () => askTerminal(F.T().path) });
    else delete actions.terminal;
  });
  addTo('pathWords', (typed, i) => {
    const here = F.T(i).path;
    const hit = pathCommand(typed, here, listing.lists.get(here)?.items ?? []);
    if (!hit) return false;
    if ('folder' in hit) void F.go(hit.folder, i);
    else void askTerminal(here, typed.trim().toLowerCase());
    return true;
  });
}

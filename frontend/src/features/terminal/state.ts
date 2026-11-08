// Whether the server lets admins open a terminal. GET /system says so, and the switch in the settings changes it.
import { ref } from 'vue';
import { admin } from '@/api/admin';
import { useUser } from '@/stores/user';

export const terminalOn = ref(false);
// The desktop app opens the computer's own terminal in a window instead of a shell in the browser.
export const onDesktop = ref(false);

export async function refreshTerminal() {
  const sys = await admin.system().catch(() => null);
  terminalOn.value = !!sys?.features.terminal;
  onDesktop.value = sys?.deployment === 'desktop';
}

export const terminalAllowed = () => terminalOn.value && useUser().admin();

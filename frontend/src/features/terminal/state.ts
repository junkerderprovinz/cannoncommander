// Whether the server lets admins open a terminal. GET /system says so, and the switch in the settings changes it.
import { ref } from 'vue';
import { admin } from '@/api/admin';
import { useUser } from '@/stores/user';

export const terminalOn = ref(false);

export async function refreshTerminal() {
  terminalOn.value = !!(await admin.system().catch(() => null))?.features.terminal;
}

export const terminalAllowed = () => terminalOn.value && useUser().admin();

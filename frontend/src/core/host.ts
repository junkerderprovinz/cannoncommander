// Where the app runs: in the container on a server, or in the desktop app on the person's own computer, and on which
// system. GET /system says so once after the sign in, and until then it is the container.
import { reactive } from 'vue';
import { admin } from '@/api/admin';
import { t } from '@/i18n';

export const host = reactive({ desktop: false, os: '' });

export async function loadHost() {
  const sys = await admin.system().catch(() => null);
  if (sys) Object.assign(host, { desktop: sys.deployment === 'desktop', os: sys.os });
}

// The top of every path, named the way the system names it: the server, or this PC on Windows and the computer
// elsewhere.
export const rootName = () => t(!host.desktop ? 'files.server' : host.os === 'windows' ? 'files.thisPc' : 'files.computer');

// In the desktop app on Windows a path reads C:\Users, as everywhere else on that computer.
export const windowsPaths = () => host.desktop && host.os === 'windows';

// Where help comes from, and the bug report: a file with the settings and the state of the jobs, without passwords,
// keys or credentials, to attach to an issue.
import { admin } from '@/api/admin';
import { prefs } from '@/core/prefs';
import { toast } from '@/core/toast';
import { c } from './kit/kit';

export const REPO = 'https://github.com/junkerderprovinz/cannoncommander';
export const MAIL = 'hello@halleluja.design';

export async function bugReport() {
  const [sys, sources, remotes, users] = await Promise.allSettled([admin.system(), admin.sources(), admin.remotes(), admin.users()]);
  const ok = <T>(r: PromiseSettledResult<T>) => (r.status === 'fulfilled' ? r.value : null);
  const report = {
    app: ok(sys)?.version ?? null,
    prefs: { ...prefs },
    sources: ok(sources)?.map((s) => ({ path: s.path, indexed: s.indexed, readOnly: s.readOnly })) ?? null,
    remotes: ok(remotes)?.map((r) => ({ type: r.type, mounted: r.mounted })) ?? null,
    users: ok(users)?.length ?? null,
  };
  const today = new Date().toISOString().slice(0, 10);
  const a = document.createElement('a');
  a.href = URL.createObjectURL(new Blob([JSON.stringify(report, null, 2)], { type: 'application/json' }));
  a.download = c('info.bugFile', { date: today });
  a.click();
  setTimeout(() => URL.revokeObjectURL(a.href), 1000);
  toast(c('info.bugSaved', { name: a.download }));
}

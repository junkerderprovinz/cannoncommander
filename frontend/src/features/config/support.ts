// Where help comes from, and the bug report: a file with the settings and the state of the jobs, without passwords,
// keys or credentials, to attach to an issue.
import { admin, type System } from '@/api/admin';
import { prefs } from '@/core/prefs';
import { toast } from '@/core/toast';
import { c } from '@/features/config/shared';

export const REPO = 'https://github.com/junkerderprovinz/cannoncommander';
export const MAIL = 'hello@halleluja.design';

// A build between releases has no release page, so it links to its commit.
export function versionLink(sys: Pick<System, 'version' | 'commit' | 'release'>): string {
  if (sys.release) return `${REPO}/releases/tag/v${sys.version.replace(/^v/, '')}`;
  return sys.commit && sys.commit !== 'unknown' ? `${REPO}/commit/${sys.commit}` : REPO;
}

export async function bugReport() {
  const [sys, sources, remotes, users] = await Promise.allSettled([admin.system(), admin.sources(), admin.remotes(), admin.users()]);
  const ok = <T>(r: PromiseSettledResult<T>) => (r.status === 'fulfilled' ? r.value : null);
  const system = ok(sys);
  const report = {
    app: system?.version ?? null,
    prefs: { ...prefs },
    os: system ? `${system.os}/${system.arch}` : null,
    sources: ok(sources)?.sources.map((s) => ({ path: s.path, indexed: s.indexed, readOnly: s.readOnly })) ?? null,
    remotes: ok(remotes)?.remotes.map((r) => ({ type: r.type, mounted: r.mounted, failed: !!r.error })) ?? null,
    rclone: ok(remotes)?.rclone ?? null,
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

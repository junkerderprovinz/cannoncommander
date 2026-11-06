// The options every new link has, for a single file as for a collection, and what the server makes of them.
import { ApiError } from '@/api/client';
import { type Share, type ShareCreate, tools } from '@/api/tools';
import { DAY, now } from '@/core/format';
import { toast } from '@/core/toast';
import { plural, t } from '@/i18n';

export interface LinkForm { slug: string; message: string; exp: string; limit: string; pw: boolean; pass: string; uploads: boolean; notify: boolean; folder: boolean }
export const MESSAGE_MAX = 2000;
export const SLUG = /^[a-z0-9][a-z0-9-]{2,39}$/;

// A few letters that are hard to guess, then the name, so the address still says what it leads to.
export function slugFor(name: string) {
  const id = Array.from({ length: 4 }, () => 'abcdefghjkmnpqrstuvwxyz23456789'[Math.floor(Math.random() * 31)]).join('');
  const words = name.toLowerCase().replace(/\.[^.]+$/, '').normalize('NFD').replace(/[^\w]+/g, '-').replace(/^-|-$/g, '').slice(0, 18);
  return `${id}-${words || 'link'}`;
}
export const newForm = (name: string, folder: boolean): LinkForm => ({ slug: slugFor(name), message: '', exp: '7', limit: '0', pw: false, pass: '', uploads: false, notify: false, folder });

// Checks the form as the server would and says what is wrong, or hands back the request.
export function readForm(f: LinkForm): Omit<ShareCreate, 'paths'> | null {
  const slug = f.slug.trim().toLowerCase(), days = Number(f.exp), message = f.message.trim();
  if (!SLUG.test(slug)) { toast(t('share.badSlug'), 'warn'); return null; }
  if (message.length > MESSAGE_MAX) { toast(plural(MESSAGE_MAX, 'share.longMessage'), 'warn'); return null; }
  if (f.pw && f.pass.length < 6) { toast(t('share.shortPassword'), 'warn'); return null; }
  return {
    slug, expires: days ? new Date(now() + days * DAY).toISOString() : null, limit: Number(f.limit), notify: f.notify,
    ...(message ? { message } : {}), ...(f.pw ? { password: f.pass } : {}), ...(f.folder && f.uploads ? { uploads: true } : {}),
  };
}

export async function create(req: ShareCreate): Promise<Share | null> {
  try {
    return await tools.share(req);
  } catch (e) {
    toast(e instanceof ApiError && e.status === 409 ? t('share.slugTaken') : (e as Error).message, 'warn');
    return null;
  }
}

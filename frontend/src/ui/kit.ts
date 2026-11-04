// What the tool windows share: counted words, copying, and the foot row that follows the window's state.
import { locale, t } from '@/i18n';
import { toast } from '@/core/toast';
import type { FootButton, WinEntry } from '@/core/windows';
import './texts';
import './bk.css';

// key names a pair of texts with one and many under it, such as ui.objects.
export const plural = (n: number, key: string) => t(`${key}.${n === 1 ? 'one' : 'many'}`, { n: n.toLocaleString(locale()) });

// The clipboard API needs a secure context, which a server on the home network often lacks, so a hidden field
// carries the text where it is missing.
export async function copyText(text: string, done = t('ui.copied')) {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    const area = Object.assign(document.createElement('textarea'), { value: text });
    area.style.cssText = 'position:fixed;opacity:0';
    document.body.append(area);
    area.select();
    document.execCommand('copy');
    area.remove();
  }
  toast(done);
}

export const setPrimary = (w: WinEntry, patch: Partial<FootButton>) => { w.foot = w.foot.map((f) => (f.primary ? { ...f, ...patch } : f)); };
// An empty start text keeps its place, so the buttons do not jump while a window fills it in.
export const setStart = (w: WinEntry, text: string) => { w.foot = w.foot.map((f) => (f.start !== undefined ? { ...f, start: text || ' ' } : f)); };

// Enter in a field of a tool window does what its primary button does.
export function pressPrimary(e: KeyboardEvent) {
  if (!(e.target as HTMLElement).closest('input')) return;
  e.preventDefault();
  (e.target as HTMLElement).closest('.win')?.querySelector<HTMLButtonElement>('.win-foot .btn.primary:not(:disabled)')?.click();
}

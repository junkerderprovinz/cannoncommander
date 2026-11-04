// What the tool windows share, as the prototype's b-00-kit.js holds it for them: counted words, copying, back and
// forward for a window that walks through folders, and the foot row that follows the window's state.
import { winNav } from '@/core/nav';
import { toast } from '@/core/toast';
import { closeBtn, type FootButton, type WinEntry } from '@/core/windows';
import { locale, t } from '@/i18n';
import './texts';
import './bk.css';

export const num = (n: number) => n.toLocaleString(locale());
// key names a pair of texts with one and many under it, such as ui.objects.
export const plural = (n: number, key: string) => t(`${key}.${n === 1 ? 'one' : 'many'}`, { n: num(n) });

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

// The mouse's side buttons step through the folders a window has shown, and closing it takes those steps back out.
export function folderSteps(w: unknown, get: () => string, set: (path: string) => void) {
  const past: string[] = [], ahead: string[] = [];
  const nav = winNav(w, {
    back: () => { if (!past.length) return; ahead.push(get()); set(past.pop()!); },
    forward: () => { if (!ahead.length) return; past.push(get()); set(ahead.pop()!); },
  });
  return {
    go(path: string) {
      if (path === get()) return;
      past.push(get());
      ahead.length = 0;
      set(path);
      nav.push();
    },
    close: nav.close,
  };
}

// A tool window's foot: the note at the start, Schließen, then its own buttons. It is rebuilt whenever the window's
// state changes, so a button's label and whether it can be pressed always match what the window shows.
export function setFoot(w: WinEntry, start: string, buttons: FootButton[]) {
  w.foot = [{ start: start || ' ' }, closeBtn(), ...buttons];
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

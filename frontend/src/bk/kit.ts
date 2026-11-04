// What the folder tools share, as the prototype's b-00-kit.js holds it for them: counted words, copying a path,
// back and forward for a window that walks through folders, and the foot row that follows a window's state.
import { winNav } from '@/core/nav';
import { toast } from '@/core/toast';
import { closeBtn, type FootButton, type WinEntry } from '@/core/windows';
import { addMessages, locale, t } from '@/i18n';
import de from './de.json';
import en from './en.json';
import './bk.css';

addMessages('bk', { de, en });

// "1 Datei", "3 Dateien": the number grouped the way the tool windows write counts, the word from the text table.
export const num = (n: number) => n.toLocaleString(locale());
export const plural = (n: number, one: string, many: string) => `${num(n)} ${n === 1 ? one : many}`;

export function copyText(text: string) {
  navigator.clipboard?.writeText(text).catch(() => {});
  toast(t('bk.copied'));
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

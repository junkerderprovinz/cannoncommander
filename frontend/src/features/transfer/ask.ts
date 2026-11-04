// A window that is a question: it resolves to the answer of the button pressed, or null when it was left.
import { cancelBtn, type FootButton, win, type WinOptions } from '@/core/windows';

export type Answer<T> = FootButton & { answer?: T };

export function ask<T>(opts: Omit<WinOptions, 'foot' | 'onClose'>, buttons: Answer<T>[], withCancel = true): Promise<T | null> {
  return new Promise((resolve) => {
    let answer: T | null = null;
    win({
      ...opts,
      foot: [...(withCancel ? [cancelBtn()] : []), ...buttons.map((b) => ({ ...b, run: (el: HTMLElement) => {
        const keep = b.run?.(el);
        if (keep !== false && b.answer !== undefined) answer = b.answer;
        return keep;
      } }))],
      onClose: () => resolve(answer),
    });
  });
}

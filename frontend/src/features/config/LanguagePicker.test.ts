import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { afterEach, beforeAll, beforeEach, describe, expect, it } from 'vitest';
import { createApp, nextTick } from 'vue';
import { prefs } from '@/core/prefs';
import { LANGUAGES } from '@/i18n';
import { SRC } from '@/testing/motionCss';
import LanguagePicker from './LanguagePicker.vue';

// jsdom draws no layout, and the open list scrolls to the language in use.
beforeAll(() => {
  Element.prototype.scrollTo ??= () => {};
  Element.prototype.scrollIntoView ??= () => {};
});
let app: ReturnType<typeof createApp> | undefined;
beforeEach(async () => {
  prefs.language = 'de';
  app = createApp(LanguagePicker);
  app.mount(document.body.appendChild(document.createElement('div')));
  for (let i = 0; i < 3; i++) await nextTick();
});
afterEach(() => {
  app?.unmount();
  document.body.innerHTML = '';
});

const trigger = () => document.querySelector<HTMLElement>('.glim-select-trigger')!;
const option = (name: string) => [...document.querySelectorAll<HTMLElement>('[role="option"]')].find((o) => o.textContent?.trim() === name);

describe('the language picker', () => {
  it('shows the language in use by its flag and its own name', () => {
    expect(trigger().querySelector('.fi.fi-de')).not.toBeNull();
    expect(trigger().querySelector('.glim-select-value > :not(.glim-select-ghost)')?.textContent).toBe('Deutsch');
  });

  it('lists every language under its own name beside a drawn flag', () => {
    for (const l of LANGUAGES) {
      const row = option(l.name);
      expect(row, l.name).toBeDefined();
      expect(row!.querySelector(`.fi.fi-${l.flag}`), l.name).not.toBeNull();
    }
    // A flag emoji is two letters on Windows, so none stands in a name.
    expect(document.body.textContent).not.toMatch(/[\u{1F1E6}-\u{1F1FF}]/u);
  });

  it('has a flag to draw for every language', () => {
    const css = readFileSync(join(SRC, 'i18n', 'flags.css'), 'utf8');
    for (const l of LANGUAGES) expect(css, l.flag).toContain(`.fi-${l.flag} {`);
  });

  it('is a field of one width whatever language is picked, as in BombVault', () => {
    const css = readFileSync(join(SRC, 'features', 'config', 'config.css'), 'utf8');
    expect(css).toMatch(/\.a-lang \.glim-select-trigger \{[^}]*width: 12rem/);
  });

  it('switches to the language picked from the list', async () => {
    trigger().click();
    await nextTick();
    const english = option('English')!;
    english.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, pointerType: 'mouse' }));
    english.click();
    await nextTick();
    expect(prefs.language).toBe('en');
  });
});

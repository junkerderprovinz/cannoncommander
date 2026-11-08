import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { type Component, createApp, nextTick } from 'vue';
import { addMessages } from '@/i18n';
import { SRC } from '@/testing/motionCss';
import de from './de.json';
import Integrations from './tabs/Integrations.vue';

const { APP, SYS } = vi.hoisted(() => ({
  APP: {
    office: { kind: 'none', url: '' }, arrowloop: { url: '' }, previews: { thumbs: true, video: false }, webdav: false, terminal: false,
    remotes: { automount: true, cache: 'off', cacheSize: 0 }, updates: { autoCheck: true }, logLevel: 'info',
  },
  SYS: { version: '1.0.0', go: '', os: '', arch: '', started: 0, uptime: 0, tools: {}, unraid: { available: false, socket: false }, features: {} },
}));
vi.mock('@/api/admin', async (original) => {
  const real = await original<typeof import('@/api/admin')>();
  return { ...real, admin: { ...real.admin, app: async () => structuredClone(APP), system: async () => structuredClone(SYS) } };
});

beforeAll(() => {
  setActivePinia(createPinia());
  addMessages('config', { de });
});
let app: ReturnType<typeof createApp> | undefined;
afterEach(() => {
  app?.unmount();
  document.body.innerHTML = '';
});
async function mount(c: Component) {
  app = createApp(c);
  app.mount(document.body.appendChild(document.createElement('div')));
  for (let i = 0; i < 5; i++) await nextTick();
}
const rule = (selector: string) => {
  const css = readFileSync(join(SRC, 'ui', 'ui.css'), 'utf8');
  const at = css.indexOf(`\n${selector} {`);
  return at < 0 ? '' : css.slice(at, css.indexOf('}', at));
};

describe('a dropdown', () => {
  it('is GlimStone\'s Select and lists its choices in plain rows, as in BombVault', async () => {
    await mount(Integrations);
    const trigger = document.querySelector('.glim-select-trigger');
    expect(trigger).not.toBeNull();
    const rows = [...document.querySelectorAll('[role="option"]')];
    expect(rows.map((r) => r.textContent?.trim())).toEqual(['Keiner', 'OnlyOffice', 'Euro-Office', 'Collabora Online']);
    expect(rows.every((r) => r.classList.contains('glim-select-item'))).toBe(true);
    expect(document.querySelector('.glim-select-content--menu')).toBeNull();
  });

  it('draws plain rows wherever the app uses one', () => {
    const vue = readdirSync(SRC, { recursive: true, encoding: 'utf8' }).filter((f) => f.endsWith('.vue'));
    const menus = vue.filter((f) => /<Select\b[^>]*\brows="menu"/.test(readFileSync(join(SRC, f), 'utf8')));
    expect(menus).toEqual([]);
  });

  it('stands on the filled ground of a text field, without a border', () => {
    const ground = rule('.glim-select-trigger:not(.glim-select-trigger--inset)');
    expect(ground).toMatch(/background: var\(--carbon-surface2\)/);
    expect(ground).toMatch(/border-color: transparent/);
    expect(rule('.glim-select-trigger:not(.glim-select-trigger--inset):hover')).toMatch(/background: var\(--carbon-surface3\)/);
  });

  it('opens a list as wide as its trigger that grows for a longer choice', () => {
    const list = rule('.glim-select-content');
    expect(list).toMatch(/min-width: var\(--reference-width, 0px\)/);
    expect(list).toMatch(/max-width: min\(92vw, 28rem\)/);
  });
});

import { afterEach, beforeAll, describe, expect, it } from 'vitest';
import { createApp, defineComponent, h, nextTick, ref } from 'vue';
import { host } from '@/core/host';
import { settingsTab, settingsTabs } from '@/core/views';
import SettingsPage from './SettingsPage.vue';

const body = (id: string) => defineComponent({ render: () => h('p', { 'data-body': id }) });

beforeAll(() => {
  settingsTabs.splice(0);
  settingsTab({ id: 'general', label: 'Allgemein', g: 'IconTabGeneral', order: 10, component: body('general') });
  settingsTab({ id: 'look', label: 'Aussehen', g: 'IconTabLook', order: 20, component: body('look') });
  settingsTab({ id: 'security', label: 'Konto', g: 'IconTabSecurity', order: 30, component: body('security'), hidden: () => host.desktop });
  settingsTab({ id: 'users', label: 'Benutzer', g: 'IconAccounts', order: 70, component: body('users'), hidden: () => host.desktop });
});

let unmount = () => {};
afterEach(() => {
  unmount();
  document.body.innerHTML = '';
  host.desktop = false;
});

async function open(arg?: string) {
  const el = document.createElement('div');
  document.body.append(el);
  const app = createApp(SettingsPage, { arg });
  app.mount(el);
  unmount = () => app.unmount();
  await nextTick();
}
const tiles = () => [...document.querySelectorAll<HTMLElement>('.stile')].map((t) => t.dataset.stab);
const shown = () => document.querySelector<HTMLElement>('[data-body]')?.dataset.body;

describe('the settings in the desktop app', () => {
  it('leave out the tiles about signing in and other people, Allgemein and Aussehen first', async () => {
    host.desktop = true;
    await open();
    expect(tiles()).toEqual(['general', 'look']);
  });

  it('open Allgemein for a link to a tile they leave out', async () => {
    host.desktop = true;
    await open('users');
    expect(shown()).toBe('general');
  });

  it('keep every tile in the container', async () => {
    await open('users');
    expect(tiles()).toEqual(['general', 'look', 'security', 'users']);
    expect(shown()).toBe('users');
  });
});

describe('the settings moving', () => {
  async function openAt(arg: string) {
    const at = ref(arg), el = document.createElement('div');
    document.body.append(el);
    const app = createApp({ render: () => h(SettingsPage, { arg: at.value }) });
    app.mount(el);
    unmount = () => app.unmount();
    await nextTick();
    return at;
  }
  const col = () => document.querySelector<HTMLElement>('#settings-col')!;

  it('slide the content in from the side of the tile picked, the trailing side for a tile further down', async () => {
    const at = await openAt('look');
    at.value = 'users';
    await nextTick();
    expect(col().classList).toContain('glim-tab-slide');
    expect(col().style.getPropertyValue('--tab-dir')).toBe('1');
    at.value = 'general';
    await nextTick();
    expect(col().style.getPropertyValue('--tab-dir')).toBe('-1');
  });

  it('bring the tiles in one after another when the page opens', async () => {
    await openAt('general');
    const tiles = [...document.querySelectorAll<HTMLElement>('.stile')];
    expect(tiles.every((t) => t.classList.contains('glim-stagger-row'))).toBe(true);
    expect(tiles.map((t) => t.style.getPropertyValue('--row-i'))).toEqual(['0', '1', '2', '3']);
  });
});

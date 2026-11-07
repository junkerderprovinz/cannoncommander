import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeAll, beforeEach, describe, expect, it } from 'vitest';
import { createApp, nextTick } from 'vue';
import { action, labelOf } from '@/core/actions';
import { closeMenu, isHead, menuState, type MenuItem } from '@/core/menu';
import { prefs } from '@/core/prefs';
import { go } from '@/core/views';
import Head from './Head.vue';
import { shell } from './state';

beforeAll(() => {
  setActivePinia(createPinia());
  const none = () => {};
  action('jobs', { label: 'Aufgaben', g: 'IconQueued', key: 'Ctrl+j', run: none });
  action('settings', { label: 'Einstellungen', g: 'IconGear', run: none });
  action('palette', { label: 'Befehle suchen', g: 'IconSearch', key: 'Ctrl+k', run: none });
  action('logout', { label: 'Abmelden', g: 'IconSignOut', run: none });
});

let app: ReturnType<typeof createApp>;
beforeEach(async () => {
  go('files');
  shell.jobCount = 0;
  prefs.dual = true;
  const el = document.createElement('div');
  document.body.append(el);
  app = createApp(Head);
  app.mount(el);
  await nextTick();
});
afterEach(() => {
  closeMenu();
  app.unmount();
  document.body.innerHTML = '';
});

const avatarMenu = () => {
  document.querySelector<HTMLElement>('.head [data-account]')!.click();
  return (menuState.list ?? []).filter((it): it is MenuItem => it !== '-' && !isHead(it));
};

describe('the head', () => {
  it('has a button for the settings, with its words, and none for the jobs', () => {
    const settings = document.querySelector('.head [data-run="settings"]');
    expect(settings?.textContent).toContain('Einstellungen');
    expect(document.querySelector('.head [data-run="jobs"]')).toBeNull();
  });

  it('keeps the jobs in the avatar menu and no second way into the settings', () => {
    const labels = avatarMenu().map((it) => labelOf(it));
    expect(labels).toContain('Aufgaben');
    expect(labels).toContain('Befehle suchen');
    expect(labels).not.toContain('Einstellungen');
    expect(labels).not.toContain('Konto & Sicherheit');
  });

  it('counts the running jobs on the avatar and on its menu entry', async () => {
    const badge = () => document.querySelector('.head [data-account] .count')!;
    expect(badge().classList).toContain('idle');
    shell.jobCount = 3;
    await nextTick();
    expect(badge().classList).not.toContain('idle');
    expect(badge().textContent).toBe('3');
    expect(avatarMenu().find((it) => labelOf(it) === 'Aufgaben')?.count).toBe(3);
  });

  it('leaves the count off the menu entry while nothing runs', () => {
    expect(avatarMenu().find((it) => labelOf(it) === 'Aufgaben')?.count).toBeUndefined();
  });

  it('names the layout that shows and switches to the other one', async () => {
    const layout = () => document.querySelector<HTMLElement>('.head .head-layout')!;
    expect(layout().textContent).toContain('Zwei Fenster');
    expect(layout().dataset.run).toBe('single');
    prefs.dual = false;
    await nextTick();
    expect(layout().textContent).toContain('Ein Fenster');
    expect(layout().dataset.run).toBe('dual');
  });
});

import { afterEach, beforeAll, beforeEach, describe, expect, it } from 'vitest';
import { createApp, h, nextTick } from 'vue';
import { action, actions } from '@/core/actions';
import { glyphComponent } from '@/core/glyphs';
import { host } from '@/core/host';
import { bindKey, keymap } from '@/core/keymap';
import { prefs } from '@/core/prefs';
import { addMessages, setLanguage } from '@/i18n';
import de from './de.json';
import en from './en.json';
import setup, { commit } from '.';
import KeysTab from './KeysTab.vue';

const SHIPPED = { F5: 'copy', F6: 'move', 'Ctrl+r': 'refresh', 'Alt+p': 'pack', 'Alt+u': 'unpack', 'Ctrl+k': 'palette', F1: 'shortcuts', 'Ctrl+t': 'newTab', 'Ctrl+n': 'newTab' };
const LABELS: Record<string, string> = { copy: 'Kopieren', move: 'Verschieben', refresh: 'Aktualisieren', pack: 'Packen', unpack: 'Entpacken', palette: 'Befehl suchen', shortcuts: 'Tastenkürzel', newTab: 'Neuer Tab', quickview: 'Schnellansicht', back: 'Zurück', gotoPath: 'Gehe zu' };

beforeAll(async () => {
  addMessages('keys', { de, en });
  setLanguage('de');
  for (const [id, label] of Object.entries(LABELS)) action(id, { label, run: () => {} });
  actions.copy!.key = 'F5';
  for (const [combo, id] of Object.entries(SHIPPED)) bindKey(combo, id);
  prefs.keys = null;
  setup();
  await Promise.resolve();
});

let app: ReturnType<typeof createApp> | undefined;
const settle = async () => { for (let i = 0; i < 4; i++) await nextTick(); };
beforeEach(async () => {
  host.desktop = false;
  commit({ ...SHIPPED }, 'cc');
  app = createApp(KeysTab);
  app.mount(document.body.appendChild(document.createElement('div')));
  await settle();
});
afterEach(() => {
  app?.unmount();
  document.body.innerHTML = '';
});

const $ = <T extends Element = HTMLElement>(sel: string) => document.querySelector<T & HTMLElement>(sel);
const rows = () => [...document.querySelectorAll('.kb-row .kb-name')].map((n) => n.textContent!.trim());
const chips = (id: string) => [...document.querySelectorAll(`[data-kb-rec="${id}"] .glim-kbd`)].map((k) => k.textContent!.trim());
const keyOf = (id: string) => $(`[data-kb-rec="${id}"]`)!.textContent!.trim();
const warning = () => $('.kb-warn')?.textContent?.trim();
async function press(key: string, mods: KeyboardEventInit = {}) {
  dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...mods }));
  await settle();
}
async function rebind(id: string) {
  $(`[data-kb-rec="${id}"]`)!.click();
  await settle();
}
// A glyph as GlimStone draws it, to tell which one a button wears.
const drawn = (name: string) => {
  const host = document.body.appendChild(document.createElement('div'));
  createApp({ render: () => h(glyphComponent(name)) }).mount(host);
  const svg = host.innerHTML;
  host.remove();
  return svg;
};
const button = (text: string) => [...document.querySelectorAll<HTMLButtonElement>('button')].find((b) => b.textContent?.trim() === text || b.getAttribute('aria-label') === text)!;

describe('the shortcuts tile', () => {
  it('starts with the layout and lists the actions under their menus', () => {
    expect([...document.querySelectorAll('.glim-notch-card > h2')].map((h) => h.textContent?.trim())[0]).toBe('Bedienung');
    expect(rows()).toContain('Kopieren');
    expect(keyOf('copy')).toBe('F5');
  });

  it('lists an action that only a row menu offers, so it can get a key, and leaves out one that needs its caller', async () => {
    action('createShortcut', { label: 'Verknüpfung erstellen', run: () => {} });
    action('openWith', { label: 'Öffnen mit', hidden: true, run: () => {} });
    await settle();
    expect(rows()).toContain('Verknüpfung erstellen');
    expect(rows()).not.toContain('Öffnen mit');
  });

  it('finds an action by its name or its key', async () => {
    const q = $<HTMLInputElement>('#kt-q')!;
    q.value = 'pack';
    q.dispatchEvent(new Event('input'));
    await settle();
    expect(rows()).toEqual(['Packen', 'Entpacken']);
    q.value = 'strg+k';
    q.dispatchEvent(new Event('input'));
    await settle();
    expect(rows()).toEqual(['Befehl suchen']);
  });

  it('gives an action the key pressed, at once and everywhere', async () => {
    await rebind('copy');
    expect(keyOf('copy')).toBe('Taste drücken …');
    await press('F5', { ctrlKey: true });
    expect(keymap['Ctrl+F5']).toBe('copy');
    expect(keymap.F5).toBeUndefined();
    expect(actions.copy!.key).toBe('Strg+F5');
    expect(prefs.keys).toEqual({ preset: 'cc', own: { 'Ctrl+F5': 'copy', F5: '' } });
  });

  it('asks before taking another action\'s key and can swap the two', async () => {
    await rebind('copy');
    await press('F6');
    expect(warning()).toContain('F6 gehört schon zu „Verschieben“');
    expect(keymap.F6).toBe('move');
    button('Tauschen').click();
    await settle();
    expect(keymap.F6).toBe('copy');
    expect(keymap.F5).toBe('move');
  });

  it('can take the key away from the other action instead', async () => {
    await rebind('copy');
    await press('F6');
    button('Ersetzen').click();
    await settle();
    expect(keymap.F6).toBe('copy');
    expect(keymap.F5).toBeUndefined();
    expect(keyOf('move')).toBe('keine');
  });

  it('leaves everything as it was on Escape', async () => {
    await rebind('copy');
    await press('F6');
    await press('Escape');
    expect(warning()).toBeUndefined();
    expect(keymap.F5).toBe('copy');
    expect(keymap.F6).toBe('move');
  });

  it('refuses a key the browser keeps, and says why', async () => {
    await rebind('copy');
    await press('w', { ctrlKey: true });
    expect(warning()).toBe('Strg+W behält der Browser für seine Tabs und Fenster. Nimm hier bitte eine andere, in der Desktop-App klappt sie.');
    expect(keymap['Ctrl+w']).toBeUndefined();
  });

  it('takes that key in the desktop app', async () => {
    host.desktop = true;
    await settle();
    await rebind('copy');
    await press('w', { ctrlKey: true });
    expect(keymap['Ctrl+w']).toBe('copy');
  });

  it('strikes out a key the browser keeps and says why in its tooltip', () => {
    const k = $('[data-kb-rec="newTab"] .kb-off')!;
    expect(k.textContent).toBe('Strg+T');
    expect(k.dataset.tip).toBe('Durchgestrichen, weil der Browser Strg+T für sich behält. Das klappt nur in der Desktop-App.');
  });

  it('explains the struck out keys in a line of the row that the key button points to', () => {
    const note = $('[data-kb="newTab"] .kb-note')!;
    expect(note.textContent!.trim()).toBe('Durchgestrichen, weil der Browser Strg+T und Strg+N für sich behält. Das klappt nur in der Desktop-App.');
    expect($('[data-kb-rec="newTab"]')!.getAttribute('aria-describedby')).toBe(note.id);
    expect($('[data-kb="copy"] .kb-note')).toBeNull();
  });

  it('strikes out nothing in the desktop app', async () => {
    host.desktop = true;
    await settle();
    expect($('.kb-off')).toBeNull();
    expect($('.kb-note')).toBeNull();
  });

  it('puts one key back as it was', async () => {
    await rebind('copy');
    await press('F5', { ctrlKey: true });
    $('[data-kb-reset="copy"]')!.click();
    await settle();
    expect(keymap.F5).toBe('copy');
    expect(prefs.keys).toBeNull();
  });

  it('asks before a reset takes its key back from another action, and can swap the two back', async () => {
    await rebind('copy');
    await press('F6');
    button('Tauschen').click();
    await settle();
    $('[data-kb-reset="copy"]')!.click();
    await settle();
    expect(warning()).toContain('F5 gehört schon zu „Verschieben“');
    expect(keymap.F5).toBe('move');
    button('Tauschen').click();
    await settle();
    expect(keymap.F5).toBe('copy');
    expect(keymap.F6).toBe('move');
    expect(prefs.keys).toBeNull();
  });

  it('lets a reset take its key back and says the other action is left without one', async () => {
    await rebind('copy');
    await press('F6');
    button('Tauschen').click();
    await settle();
    $('[data-kb-reset="copy"]')!.click();
    await settle();
    button('Ersetzen').click();
    await settle();
    expect(keymap.F5).toBe('copy');
    expect(keyOf('move')).toBe('keine');
  });

  it('puts every key back, and only offers to once there is something to put back', async () => {
    const all = $<HTMLButtonElement>('#kt-reset')!;
    expect(all.disabled).toBe(true);
    await rebind('copy');
    await press('F5', { ctrlKey: true });
    expect(all.disabled).toBe(false);
    all.click();
    await settle();
    expect(keymap.F5).toBe('copy');
    expect(keymap['Ctrl+F5']).toBeUndefined();
  });

  it('offers no layout in the container and the first start\'s two in the desktop app', async () => {
    const layouts = () => [...document.querySelectorAll('#kt-layout button')].map((b) => b.textContent?.trim());
    expect($('#kt-layout')).toBeNull();
    host.desktop = true;
    await settle();
    expect(layouts()).toEqual(['Wie ein Commander', 'Wie im Explorer']);
    expect($('#d-clickselect')).not.toBeNull();
  });

  it('brings Explorer\'s keys when it is chosen in the desktop app', async () => {
    host.desktop = true;
    await settle();
    [...document.querySelectorAll<HTMLButtonElement>('#kt-layout button')].find((b) => b.textContent?.trim() === 'Wie im Explorer')!.click();
    await settle();
    expect(keymap.F5).toBe('refresh');
    expect(chips('refresh')).toEqual(['F5', 'Strg+R']);
  });

  it('shows every key of an action as a chip of its own', () => {
    expect(chips('newTab')).toEqual(['Strg+T', 'Strg+N']);
    expect(keyOf('newTab')).not.toContain('+1');
  });

  it('puts a changed key back with a reset button named for it', async () => {
    await rebind('copy');
    await press('F5', { ctrlKey: true });
    const back = $('[data-kb-reset="copy"]')!;
    expect(back.getAttribute('aria-label')).toBe('Zurücksetzen');
    expect(back.querySelector('svg')?.outerHTML).toBe(drawn('IconResetArrow'));
    back.click();
    await settle();
    expect(keymap.F5).toBe('copy');
  });

  it('deletes a key with a trash button named for it', async () => {
    const del = $('[data-kb-clear="copy"]')!;
    expect(del.getAttribute('aria-label')).toBe('Taste löschen');
    expect(del.querySelector('svg')?.outerHTML).toBe(drawn('IconTrash'));
    del.click();
    await settle();
    expect(keymap.F5).toBeUndefined();
    expect(keyOf('copy')).toBe('keine');
  });
});

// Every folder under features/ with an index.ts is one feature. Its texts sit beside it in de.json and en.json and
// are reachable under the folder's name, so features never share a text table.
//
// What only the desktop app does goes into a desktop.ts, beside the index.ts or in its place. The container build
// leaves those files out, and the desktop app loads them once the server says it is the desktop app.
import { watch } from 'vue';
import { host } from '@/core/host';
import { addMessages } from '@/i18n';

type Texts = Record<string, Record<string, unknown>>;
type Setup = { default: () => void };
const texts = import.meta.glob<Texts>('./*/{de,en}.json', { eager: true, import: 'default' });
const modules = import.meta.glob<Setup>('./*/index.ts', { eager: true });
const desktopModules = import.meta.env.DESKTOP ? import.meta.glob<Setup>('./*/desktop.ts') : {};

export function loadFeatures() {
  const byFeature: Record<string, Record<string, Texts>> = {};
  for (const [file, table] of Object.entries(texts)) {
    const [, name, lang] = file.match(/^\.\/([^/]+)\/(de|en)\.json$/)!;
    (byFeature[name!] ??= {})[lang!] = table;
  }
  for (const [name, t] of Object.entries(byFeature)) addMessages(name, t as never);
  for (const mod of Object.values(modules)) mod.default();
  if (import.meta.env.DESKTOP) watch(() => host.desktop, (desktop) => { if (desktop) void loadDesktopFeatures(); }, { immediate: true });
}

let desktopLoaded: Promise<void> | undefined;

export function loadDesktopFeatures() {
  desktopLoaded ??= Promise.all(Object.values(desktopModules).map((load) => load())).then((mods) => {
    for (const mod of mods) mod.default();
  });
  return desktopLoaded;
}

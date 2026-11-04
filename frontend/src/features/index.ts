// Every folder under features/ with an index.ts is one feature. Its texts sit beside it in de.json and en.json and
// are reachable under the folder's name, so features never share a text table.
import { addMessages } from '@/i18n';

type Texts = Record<string, Record<string, unknown>>;
const texts = import.meta.glob<Texts>('./*/{de,en}.json', { eager: true, import: 'default' });
const modules = import.meta.glob<{ default: () => void }>('./*/index.ts', { eager: true });

export function loadFeatures() {
  const byFeature: Record<string, Record<string, Texts>> = {};
  for (const [file, table] of Object.entries(texts)) {
    const [, name, lang] = file.match(/^\.\/([^/]+)\/(de|en)\.json$/)!;
    (byFeature[name!] ??= {})[lang!] = table;
  }
  for (const [name, t] of Object.entries(byFeature)) addMessages(name, t as never);
  for (const mod of Object.values(modules)) mod.default();
}

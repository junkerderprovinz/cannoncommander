// The tile Kontextmenü, which hides and moves the rows of the app's own menus in both builds. The computer's own menu,
// its properties dialog, Als Administrator ausführen and its icons come with desktop.ts.
import './shellmenu.css';
import { settingsTab } from '@/core/views';
import { type MenuShaper, shapeMenus } from '@/files/menus';
import { tl } from '@/i18n';
import { shape } from './custom';
import MenuTab from './MenuTab.vue';

// The system's row joins a menu before the person's shape applies, so it hides and moves like the app's own rows.
let system: MenuShaper | undefined;
export const reachSystemMenu = (fn: MenuShaper) => { system = fn; };

export default function setup() {
  shapeMenus((kind, items, at) => {
    const list = system ? system(kind, items, at) : items;
    return list && shape(kind, list);
  });
  settingsTab({ id: 'ctxmenu', label: tl('shellmenu.tile'), g: 'IconMenu', order: 45, component: MenuTab });
}

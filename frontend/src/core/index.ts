// The core services under the names the prototype's CC object uses, so ported feature code and the test shim find them.
export { action, actions, blocked, labelOf, run, setActionContext } from './actions';
export { say } from './announce';
export { addTo, extend } from './extend';
export { fmtAgo, fmtDate, fmtListDate, fmtSize, fmtTime, now, setClock } from './format';
export { G, glyph, glyphOf, kindOf } from './glyphs';
export { handleKey, installKeyboard } from './keyboard';
export { bindKey, comboOf, keymap } from './keymap';
export { closeMenu, menu, resolveItems } from './menu';
export { MENUS, menuItems, openMenuBar } from './menus';
export { Nav, pushNav, winNav } from './nav';
export { applyPrefs, prefs, setPref } from './prefs';
export { toast } from './toast';
export { go, registerView, settingsTab, view, views } from './views';
export { closeBtn, cancelBtn, closeWin, win, winStack } from './windows';

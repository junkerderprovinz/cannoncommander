import { reactive } from 'vue';

// Auswahlmodus: while it is on, every row of the folder it started in offers a box to tick.
export const touchSel = reactive({ on: false, path: '' });
export const ticking = (path: string) => touchSel.on && touchSel.path === path;

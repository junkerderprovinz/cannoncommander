import { ListRow, Pill } from '@glimstone/vue';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { afterEach, describe, expect, it } from 'vitest';
import { createApp, h } from 'vue';

const SRC = join(import.meta.dirname, '..', '..');
const sheet = (path: string) => document.head.appendChild(Object.assign(document.createElement('style'), { textContent: readFileSync(path, 'utf8') }));

afterEach(() => {
  document.head.innerHTML = '';
  document.body.innerHTML = '';
});

// A flex row lays its children out by their order and then as they stand in the markup.
const laidOut = (row: Element) => [...row.children]
  .map((el, at) => ({ el, at, order: Number(getComputedStyle(el).order) || 0 }))
  .sort((a, b) => a.order - b.order || a.at - b.at)
  .map(({ el }) => el.className.replace(/^glim-list-row-(\w+).*$/, '$1'));

describe('a settings row', () => {
  it('keeps its values at the right edge and brings its buttons in to their left', () => {
    sheet(join(SRC, '..', 'node_modules', '@glimstone', 'core', 'tokens.css'));
    sheet(join(SRC, 'features', 'config', 'config.css'));
    const host = document.body.appendChild(Object.assign(document.createElement('div'), { className: 'a-list' }));
    createApp({
      render: () => h(ListRow, { name: 'Shares' }, {
        tile: () => h('span'),
        badges: () => h(Pill, null, () => '/mnt/user'),
        actions: () => h('button', { type: 'button' }, 'edit'),
      }),
    }).mount(host);
    expect(laidOut(host.querySelector('.glim-list-row')!)).toEqual(['tile', 'name', 'actions', 'badges']);
  });
});

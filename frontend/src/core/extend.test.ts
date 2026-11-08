import { describe, expect, it } from 'vitest';
import { addTo, extend, SIDE_ROWS } from './extend';

describe('addTo', () => {
  it('puts a row that comes later at the place its order names', () => {
    const parity = () => 'parity', collection = () => 'collection', recent = () => 'recent', plain = () => 'plain';
    addTo('sideRows', parity, SIDE_ROWS.parity);
    addTo('sideRows', collection, SIDE_ROWS.collection);
    addTo('sideRows', plain);
    const remove = addTo('sideRows', recent, SIDE_ROWS.recent);
    expect(extend.sideRows.map((row) => row())).toEqual(['plain', 'recent', 'collection', 'parity']);
    remove();
    expect(extend.sideRows.map((row) => row())).toEqual(['plain', 'collection', 'parity']);
  });
});

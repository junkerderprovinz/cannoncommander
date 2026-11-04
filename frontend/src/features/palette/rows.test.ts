import { describe, expect, it } from 'vitest';
import { score } from './rows';

describe('command search', () => {
  it('ranks the start of a word above a hit inside a word', () => {
    expect(score('ord', { name: 'Neuer Ordner' })).toBeGreaterThan(score('ord', { name: 'Akkord' }));
  });
  it('wants every typed word', () => {
    expect(score('neu ord', { name: 'Neuer Ordner' })).toBeGreaterThan(0);
    expect(score('neu xyz', { name: 'Neuer Ordner' })).toBe(0);
  });
  it('folds umlauts and ß', () => {
    expect(score('losch', { name: 'Löschen' })).toBeGreaterThan(0);
    expect(score('grosse', { name: 'Größe' })).toBeGreaterThan(0);
  });
  it('finds a word in the menu a label sits under', () => {
    expect(score('sortieren', { name: 'Name', group: 'Sortieren' })).toBe(1);
  });
});

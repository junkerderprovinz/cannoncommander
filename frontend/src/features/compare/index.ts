// Ordner vergleichen (Strg+Umschalt+C) and Synchronisieren between the two panels.
import { action } from '@/core/actions';
import { tl } from '@/i18n';
import { compareDirs, syncWindow } from './sync';
import './compare.css';

export default function setup() {
  action('compareDirs', { label: tl('compare.title'), g: 'IconCompare', key: 'Strg+Umschalt+C', needs: 'dual', run: compareDirs });
  action('sync', { label: tl('compare.sync'), g: 'IconSync', needs: 'dual', run: syncWindow });
}

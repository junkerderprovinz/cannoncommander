// Ordner vergleichen (Strg+Umschalt+C) and Synchronisieren between the two panels.
import { action } from '@/core/actions';
import { kindLabels, placeName } from '@/features/jobs/label';
import { t, tl } from '@/i18n';
import { compareDirs, syncWindow } from './sync';
import './compare.css';

export default function setup() {
  action('compareDirs', { label: tl('compare.title'), g: 'IconCompare', key: 'Ctrl+Shift+c', needs: 'dual', run: compareDirs });
  action('sync', { label: tl('compare.sync'), g: 'IconSync', needs: 'dual', run: syncWindow });
  kindLabels.sync = (j) => t('compare.job', { name: j.name, to: placeName(j.dest) });
}

// Hilfe > Glyphen für Dateien: the glyphs the file view uses, and the kinds GlimStone has none for yet.
import { action } from '@/core/actions';
import { go, registerView } from '@/core/views';
import { tl } from '@/i18n';
import GlyphsPage from './GlyphsPage.vue';

export default function setup() {
  registerView('glyphs', { page: true, component: GlyphsPage });
  action('glyphs', { label: tl('glyphs.title'), g: 'IconFiles', run: () => go('glyphs') });
}

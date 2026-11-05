<script setup lang="ts">
// Link erstellt: the address with its QR code, what the link allows, and for a collection what the other side sees.
import { InfoBubble } from '@glimstone/vue';
import { computed, onMounted, ref } from 'vue';
import { type Share, shareUrl, tools } from '@/api/tools';
import { fmtDate, fmtSize } from '@/core/format';
import { G, glyphOf } from '@/core/glyphs';
import { t } from '@/i18n';
import CopyBtn from '@/ui/CopyBtn.vue';
import Frow from '@/ui/Frow.vue';
import { plural } from '@/ui/kit';
import { asFolder, type Collected } from './basket';

const props = defineProps<{ link: Share; dir: boolean; items?: Collected[] }>();
const url = ref(shareUrl(props.link.slug)), qr = ref('');
const field = ref<HTMLInputElement | null>(null);
onMounted(() => {
  tools.address(props.link.id).then((a) => { url.value = a.url; qr.value = a.qr; }, () => {});
  setTimeout(() => field.value?.select(), 40);
});
const shown = computed(() => (props.items ? asFolder(props.items) : []));
const yes = (on: boolean) => (on ? t('share.yes') : t('share.no'));
</script>

<template>
  <div class="well glim-well cs-done"><div class="cs-qr" role="img" :aria-label="t('share.qr')" v-html="qr"></div><div class="cs-facts">
    <div class="cs-url"><input ref="field" class="glim-input grow glim-input--mono" :value="url" readonly :aria-label="t('share.link')"><CopyBtn :text="url" :tip="t('share.copyLink')" :done="t('share.linkCopied')" /></div>
    <Frow :label="t('share.for')"><span class="pill">{{ link.name }}</span></Frow>
    <Frow :label="t('share.valid')"><span class="pill glim-num">{{ link.expires ? t('share.until', { date: fmtDate(Date.parse(link.expires)) }) : t('share.unlimited') }}</span></Frow>
    <Frow :label="t('share.downloads')"><span class="pill glim-num">{{ link.limit ? t('share.atMost', { n: link.limit }) : t('share.noLimit') }}</span></Frow>
    <Frow :label="t('share.password')"><span class="pill">{{ yes(link.password) }}</span></Frow>
    <Frow v-if="dir && !items" :label="t('share.upload')"><span class="pill">{{ link.uploads ? t('share.allowed') : t('share.no') }}</span></Frow>
    <Frow :label="t('share.notify')"><span class="pill">{{ yes(link.notify) }}</span></Frow>
  </div></div>
  <div v-if="items" class="well glim-well"><div class="eyebrow">{{ t('share.visitor') }} <InfoBubble :tip="t('share.visitorTip')" /></div>
    <div class="cs-visitor"><div class="cs-vhead"><G n="IconFolderOpen" /><b>{{ link.name }}</b><span class="muted">{{ plural(items.length, 'ui.objects') }}</span></div>
      <div v-for="x in shown.slice(0, 8)" :key="x.path" class="cs-vrow"><G :n="glyphOf(x)" /><span>{{ x.shown }}</span><span class="muted glim-num">{{ fmtSize(x.size) || '0 B' }}</span></div>
      <div v-if="items.length > 8" class="cs-vrow muted">{{ t('share.more', { n: items.length - 8 }) }}</div>
    </div>
  </div>
</template>

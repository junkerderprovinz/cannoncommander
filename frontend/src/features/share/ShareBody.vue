<script setup lang="ts">
// Freigeben for one file or folder: the options of a new link, and the links it already has.
import { type Share, shareUrl } from '@/api/tools';
import { fmtDate, fmtSize } from '@/core/format';
import { G, glyphOf } from '@/core/glyphs';
import type { Entry } from '@/files/entries';
import { t } from '@/i18n';
import CopyBtn from '@/ui/CopyBtn.vue';
import Frow from '@/ui/Frow.vue';
import { plural } from '@/ui/kit';
import type { LinkForm } from './link';
import LinkOptions from './LinkOptions.vue';

defineProps<{ k: Entry; f: LinkForm; st: { existing: Share[] }; reroll: (s: Share) => void }>();
</script>

<template>
  <div class="well"><Frow><template #label><span class="tile cs-tile"><G :n="glyphOf(k)" /></span><b class="cs-name">{{ k.name }}</b></template><span class="pill glim-num">{{ fmtSize(k.size) || '0 B' }}</span></Frow></div>
  <LinkOptions :f="f" :name="k.name" />
  <div v-if="st.existing.length" id="cs-existing" class="well"><div class="eyebrow">{{ t('share.existing') }}</div>
    <Frow v-for="s in st.existing" :key="s.id"><template #label><span class="mono cs-slug">{{ shareUrl(s.slug) }}</span></template>
      <span class="pill glim-num">{{ s.expires ? t('share.until', { date: fmtDate(Date.parse(s.expires)) }) : t('share.unlimited') }}</span><span class="pill glim-num">{{ plural(s.views, 'share.views') }}</span>
      <button class="btn icon" type="button" :data-reroll="s.slug" :data-tip="t('share.reroll')" :aria-label="t('share.dice')" @click="reroll(s)"><G n="IconRetry" /></button>
      <CopyBtn :text="shareUrl(s.slug)" :tip="t('share.copyLink')" :done="t('share.linkCopied')" />
    </Frow>
  </div>
</template>

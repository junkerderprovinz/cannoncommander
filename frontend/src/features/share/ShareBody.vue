<script setup lang="ts">
// Freigeben for one file or folder: the options of a new link, and the links it already has.
import { FieldRow, IconButton, Pill } from '@glimstone/vue';
import { type Share, shareUrl } from '@/api/tools';
import { fmtDate, fmtSize } from '@/core/format';
import { G, glyphOf, glyphComponent } from '@/core/glyphs';
import type { Entry } from '@/files/entries';
import { t } from '@/i18n';
import CopyBtn from '@/ui/CopyBtn.vue';
import { plural } from '@/ui/kit';
import type { LinkForm } from './link';
import LinkOptions from './LinkOptions.vue';

defineProps<{ k: Entry; f: LinkForm; st: { existing: Share[] }; reroll: (s: Share) => void }>();
</script>

<template>
  <div class="well glim-well"><FieldRow :label="k.name"><template #label><span class="cs-head"><span class="tile cs-tile"><G :n="glyphOf(k)" /></span><b class="cs-name">{{ k.name }}</b></span></template><Pill class="glim-num">{{ fmtSize(k.size) || '0 B' }}</Pill></FieldRow></div>
  <LinkOptions :f="f" :name="k.name" />
  <div v-if="st.existing.length" id="cs-existing" class="well glim-well"><div class="eyebrow">{{ t('share.existing') }}</div>
    <FieldRow v-for="s in st.existing" :key="s.id" :label="shareUrl(s.slug)"><template #label><span class="mono cs-slug">{{ shareUrl(s.slug) }}</span></template>
      <span class="a-ctl"><Pill class="glim-num">{{ s.expires ? t('share.until', { date: fmtDate(Date.parse(s.expires)) }) : t('share.unlimited') }}</Pill><Pill class="glim-num">{{ plural(s.views, 'share.views') }}</Pill><IconButton :tip="t('share.reroll')" :glyph="glyphComponent('IconRetry')" :data-reroll="s.slug" @click="reroll(s)" /><CopyBtn :text="shareUrl(s.slug)" :tip="t('share.copyLink')" :done="t('share.linkCopied')" /></span>
    </FieldRow>
  </div>
</template>

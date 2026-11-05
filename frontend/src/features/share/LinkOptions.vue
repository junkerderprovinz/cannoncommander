<script setup lang="ts">
import { eyeWords, segItems } from '@/ui/kit';
import { FieldRow, IconButton, PasswordField, Selector, TextField, Toggle } from '@glimstone/vue';
import { computed, nextTick, watch } from 'vue';
import { glyphComponent } from '@/core/glyphs';
import { t } from '@/i18n';
import { type LinkForm, slugFor } from './link';

const props = defineProps<{ f: LinkForm; name: string }>();
const host = location.host + '/s/';
const expiry = computed(() => ['1', '7', '30', '0'].map((d) => [d, t(`share.expiry.${d}`)] as const));
const limits = computed(() => [['0', t('share.none')], ['1', '1'], ['5', '5'], ['10', '10'], ['25', '25']] as const);
watch(() => props.f.pw, async (on) => { if (on) { await nextTick(); document.getElementById('cs-pass')?.focus(); } });
</script>

<template>
  <div class="well glim-well"><div class="eyebrow">{{ t('share.newLink') }}</div>
    <FieldRow :label="t('share.address')" :hint="t('share.addressTip')">
      <span class="cs-slugf"><span class="muted mono cs-host">{{ host }}</span><TextField id="cs-slug" v-model="f.slug" autocomplete="off" spellcheck="false" :aria-label="t('share.addressEnd')" mono /><IconButton id="cs-dice" :tip="t('share.dice')" :glyph="glyphComponent('IconRetry')" @click="f.slug = slugFor(name)" /></span>
    </FieldRow>
    <FieldRow :label="t('share.valid')" :hint="t('share.validTip')"><Selector id="cs-exp" v-model="f.exp" :items="segItems(expiry)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    <FieldRow :label="t('share.limit')" :hint="t('share.limitTip')"><Selector id="cs-limit" v-model="f.limit" :items="segItems(limits)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    <FieldRow :label="t('share.withPassword')"><Toggle id="cs-pw" v-model="f.pw" /></FieldRow>
    <div id="cs-pwrow" :hidden="!f.pw">
      <FieldRow :label="t('share.password')"><PasswordField id="cs-pass" v-model="f.pass" :placeholder="t('share.passwordPlaceholder')" autocomplete="new-password" width="wide" :text="eyeWords()" /></FieldRow>
    </div>
    <FieldRow v-if="f.folder" :label="t('share.uploads')" :hint="t('share.uploadsTip')"><Toggle id="cs-up" v-model="f.uploads" /></FieldRow>
    <FieldRow :label="t('share.notify')" :hint="t('share.notifyTip')"><Toggle id="cs-notify" v-model="f.notify" /></FieldRow>
  </div>
</template>

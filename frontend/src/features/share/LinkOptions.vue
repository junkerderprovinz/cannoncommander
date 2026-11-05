<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue';
import { G } from '@/core/glyphs';
import { t } from '@/i18n';
import Frow from '@/ui/Frow.vue';
import Seg from '@/ui/Seg.vue';
import Sw from '@/ui/Sw.vue';
import { type LinkForm, slugFor } from './link';

const props = defineProps<{ f: LinkForm; name: string }>();
const host = location.host + '/s/';
const expiry = computed(() => ['1', '7', '30', '0'].map((d) => [d, t(`share.expiry.${d}`)] as const));
const limits = computed(() => [['0', t('share.none')], ['1', '1'], ['5', '5'], ['10', '10'], ['25', '25']] as const);
const pass = ref<HTMLInputElement | null>(null), shown = ref(false);
watch(() => props.f.pw, async (on) => { if (on) { await nextTick(); pass.value?.focus(); } });
</script>

<template>
  <div class="well glim-well"><div class="eyebrow">{{ t('share.newLink') }}</div>
    <Frow :label="t('share.address')" :tip="t('share.addressTip')">
      <span class="cs-slugf"><span class="muted mono cs-host">{{ host }}</span><input id="cs-slug" v-model="f.slug" class="glim-input glim-input--mono" autocomplete="off" spellcheck="false" :aria-label="t('share.addressEnd')"><button id="cs-dice" class="btn icon" type="button" :data-tip="t('share.dice')" :aria-label="t('share.dice')" @click="f.slug = slugFor(name)"><G n="IconRetry" /></button></span>
    </Frow>
    <Frow :label="t('share.valid')" :tip="t('share.validTip')"><Seg id="cs-exp" v-model="f.exp" :options="expiry" /></Frow>
    <Frow :label="t('share.limit')" :tip="t('share.limitTip')"><Seg id="cs-limit" v-model="f.limit" :options="limits" /></Frow>
    <Frow :label="t('share.withPassword')"><Sw id="cs-pw" v-model="f.pw" /></Frow>
    <div id="cs-pwrow" :hidden="!f.pw">
      <Frow :label="t('share.password')"><span class="cs-secret"><input id="cs-pass" ref="pass" v-model="f.pass" class="glim-input w" :type="shown ? 'text' : 'password'" :placeholder="t('share.passwordPlaceholder')" autocomplete="new-password"><button class="cs-eye" type="button" data-eye :data-tip="shown ? t('ui.hide') : t('ui.show')" :aria-label="shown ? t('ui.hide') : t('ui.show')" @click="shown = !shown"><G :n="shown ? 'IconEyeOff' : 'IconEye'" /></button></span></Frow>
    </div>
    <Frow v-if="f.folder" :label="t('share.uploads')" :tip="t('share.uploadsTip')"><Sw id="cs-up" v-model="f.uploads" /></Frow>
    <Frow :label="t('share.notify')" :tip="t('share.notifyTip')"><Sw id="cs-notify" v-model="f.notify" /></Frow>
  </div>
</template>

<script setup lang="ts">
// A password field that grows with its row, with the eye that shows what was typed.
import { ref } from 'vue';
import { G } from '@/core/glyphs';
import { t } from '@/i18n';
import './texts';

defineProps<{ id?: string; placeholder?: string; autocomplete?: string }>();
const model = defineModel<string>({ default: '' });
const show = ref(false);
</script>

<template>
  <span class="pw-field"><input :id="id" v-model="model" :type="show ? 'text' : 'password'" class="field" :autocomplete="autocomplete" :placeholder="placeholder" spellcheck="false"><button class="pw-eye" type="button" tabindex="-1" :data-tip="show ? t('ui.hide') : t('ui.show')" :aria-label="show ? t('ui.hide') : t('ui.show')" @click="show = !show"><G :n="show ? 'IconEyeOff' : 'IconEye'" /></button></span>
</template>

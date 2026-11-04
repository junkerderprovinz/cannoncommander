<script setup lang="ts">
// A password field with the eye that shows what was typed. GlimStone's PasswordField has the same eye but sets the
// field on the lighter ground, so this keeps the prototype's field.
import { ref } from 'vue';
import { G } from '@/core/glyphs';
import { c } from './kit';

withDefaults(defineProps<{ size?: string; placeholder?: string; autocomplete?: string }>(), { size: 'w', autocomplete: 'new-password' });
const model = defineModel<string>({ default: '' });
const show = ref(false);
const input = ref<HTMLInputElement | null>(null);
defineExpose({ input });
</script>

<template>
  <span class="a-pw"><input ref="input" v-model="model" :type="show ? 'text' : 'password'" class="field" :class="size" :autocomplete="autocomplete" :placeholder="placeholder"><button class="a-eye" type="button" :data-tip="show ? c('hide') : c('show')" :aria-label="show ? c('hide') : c('show')" @click="show = !show"><G :n="show ? 'IconEyeOff' : 'IconEye'" /></button></span>
</template>

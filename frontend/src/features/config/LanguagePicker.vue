<script setup lang="ts">
import { Select } from '@glimstone/vue/Select';
import { computed, type FunctionalComponent, h, onMounted } from 'vue';
import { prefs, setPref } from '@/core/prefs';
import { language as current, languages } from '@/i18n';

// The flags weigh half a megabyte, so they load with the first list that shows them and not with the app.
onMounted(() => import('@/i18n/flags.css'));

// Before anyone picks a language the list shows the one the browser asked for, and saves nothing.
const language = computed({ get: () => prefs.language || current(), set: (v: string) => setPref('language', v) });
const flag = (code: string): FunctionalComponent => () => h('span', { class: `fi fi-${code}` });
const options = computed(() => languages().map((l) => ({ value: l.code, label: l.name, glyph: l.flag ? flag(l.flag) : undefined })));
</script>

<template>
  <Select v-model="language" class="a-lang" list-id="a-langs" :options="options" />
</template>

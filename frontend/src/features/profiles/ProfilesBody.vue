<script setup lang="ts">
// The saved layouts with a button to load or drop each, and the name for the one to save next.
import type { Profile } from '@/api/tools';
import { G } from '@/core/glyphs';
import { split } from '@/files/paths';
import { t } from '@/i18n';
import Frow from '@/ui/Frow.vue';

defineProps<{ st: { list: Profile[]; name: string }; load: (p: Profile) => void; drop: (p: Profile) => void }>();
const tabsOf = (p: Profile) => p.panels.map((x) => x.tabs.map((path) => split(path).pop() || t('files.server')).join(', ')).join(' | ');
const enter = (e: KeyboardEvent) => (e.target as Element).closest('.win')?.querySelector<HTMLElement>('.win-foot .btn.primary')?.click();
</script>

<template>
  <div class="well">
    <div class="eyebrow">{{ t('profiles.saved') }}</div>
    <template v-if="st.list.length">
      <div v-for="p in st.list" :key="p.id" class="frow">
        <span class="lab"><G n="IconDashboard" /><b>{{ p.name }}</b><small>{{ tabsOf(p) }}</small></span>
        <button class="btn" :data-pload="p.id" @click="load(p)"><G n="IconUpload" />{{ t('profiles.load') }}</button>
        <button class="btn icon" :data-pdel="p.id" :data-tip="t('profiles.delete')" :aria-label="t('profiles.delete')" @click="drop(p)"><G n="IconTrash" /></button>
      </div>
    </template>
    <div v-else class="muted">{{ t('profiles.none') }}</div>
  </div>
  <div class="well">
    <Frow :label="t('profiles.name')" :tip="t('profiles.hint')"><input id="pname" v-model="st.name" class="field m" :placeholder="t('profiles.placeholder')" autocomplete="off" @keydown.enter="enter"></Frow>
  </div>
</template>

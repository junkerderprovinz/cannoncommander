<script setup lang="ts">
// The saved layouts with a button to load or drop each, and the name for the one to save next.
import { Button, FieldRow, IconButton, TextField } from '@glimstone/vue';
import type { Profile } from '@/api/tools';
import { G, glyphComponent } from '@/core/glyphs';
import { split } from '@/files/paths';
import { t } from '@/i18n';

defineProps<{ st: { list: Profile[]; name: string }; load: (p: Profile) => void; drop: (p: Profile) => void }>();
const tabsOf = (p: Profile) => p.panels.map((x) => x.tabs.map((path) => split(path).pop() || t('files.server')).join(', ')).join(' | ');
const enter = (e: KeyboardEvent) => (e.target as Element).closest('.win')?.querySelector<HTMLElement>('.glim-modal-foot .glim-btn--accent')?.click();
</script>

<template>
  <div class="well glim-well">
    <div class="eyebrow">{{ t('profiles.saved') }}</div>
    <template v-if="st.list.length">
      <FieldRow v-for="p in st.list" :key="p.id" :label="p.name">
        <template #label><span class="pf-name"><G n="IconDashboard" /><b>{{ p.name }}</b><small>{{ tabsOf(p) }}</small></span></template>
        <span class="a-ctl">
        <Button :label="t('profiles.load')" label-key="IconUpload" :data-pload="p.id" @click="load(p)" />
        <IconButton :tip="t('profiles.delete')" :glyph="glyphComponent('IconTrash')" :data-pdel="p.id" @click="drop(p)" />
        </span>
      </FieldRow>
    </template>
    <div v-else class="muted">{{ t('profiles.none') }}</div>
  </div>
  <div class="well glim-well">
    <FieldRow :label="t('profiles.name')" :hint="t('profiles.hint')"><TextField id="pname" v-model="st.name" :placeholder="t('profiles.placeholder')" autocomplete="off" width="medium" @keydown.enter="enter" /></FieldRow>
  </div>
</template>

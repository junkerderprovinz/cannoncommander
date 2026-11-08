<script setup lang="ts">
// Neue Verknüpfung: the target as the computer writes it, and the name the shortcut gets.
import { FieldRow, FieldRowGroup, SubCard, TextField } from '@glimstone/vue';
import { t } from '@/i18n';
import { pressPrimary } from '@/ui/kit';

export interface ShortcutState { target: string; name: string }
defineProps<{ st: ShortcutState }>();
</script>

<template>
  <SubCard class="bk-form" @keydown.enter="pressPrimary">
    <FieldRowGroup>
      <FieldRow :label="t('filesystem.target')" :hint="t('filesystem.targetTip')"><TextField id="sc-target" v-model="st.target" spellcheck="false" autocomplete="off" width="grow" mono /></FieldRow>
      <FieldRow :label="t('filesystem.name')"><TextField id="sc-name" v-model="st.name" spellcheck="false" autocomplete="off" width="grow" /></FieldRow>
    </FieldRowGroup>
  </SubCard>
</template>

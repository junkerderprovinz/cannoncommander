<script setup lang="ts">
// Mehr Details and Weniger Details, at the start of a job window's foot.
import { G } from '@/core/glyphs';
import { prefs } from '@/core/prefs';
import { t } from '@/i18n';
import { toggleDetails } from './details';
</script>

<template>
  <button type="button" class="jf-toggle" :aria-expanded="prefs.jobDetails === true" @click="toggleDetails">
    <G n="IconExpand" />{{ prefs.jobDetails ? t('jobs.lessDetails') : t('jobs.moreDetails') }}
  </button>
</template>

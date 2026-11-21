<script setup lang="ts">
// Konflikte: the name asked about and both versions of it side by side, a picture of each for an image, with what is
// newer or larger called out. The foot's buttons answer it; Für alle übernehmen lets one of them answer every name left.
import { Badge, Checkbox } from '@glimstone/vue';
import { computed, ref, watch } from 'vue';
import { api } from '@/api/client';
import type { ConflictSide, JobConflict } from '@/api/ops';
import { fmtSize, fmtTime } from '@/core/format';
import { G, glyphOf, kindOf } from '@/core/glyphs';
import { split } from '@/files/paths';
import { plural, t } from '@/i18n';

const props = defineProps<{ conflicts: JobConflict[]; dest: string; st: { at: number; all: boolean }; later?: boolean }>();
const c = computed(() => props.conflicts[props.st.at]!);
const many = computed(() => props.conflicts.length > 1);
const more = computed(() => props.later || props.conflicts.length - props.st.at > 1);
const name = computed(() => split(c.value.key).pop() || c.value.key);
const image = computed(() => !c.value.source.dir && kindOf({ name: name.value }) === 'img');
const glyph = computed(() => glyphOf({ name: name.value, dir: c.value.source.dir }));
const newer = (a: ConflictSide, b: ConflictSide) => Date.parse(a.mtime) > Date.parse(b.mtime) + 60000;
const size = (n: ConflictSide) => (n.dir ? plural(n.entries ?? 0, 'transfer.entries') + ' · ' : '') + (fmtSize(n.size) || '0 B');
const sides = computed(() => [
  { key: 'source', label: t('transfer.sideNew'), side: c.value.source, bigger: c.value.source.size > c.value.target.size, newer: newer(c.value.source, c.value.target) },
  { key: 'target', label: t('transfer.sideOld'), side: c.value.target, bigger: c.value.target.size > c.value.source.size, newer: newer(c.value.target, c.value.source) },
]);
// A picture that cannot be made, such as one the server has no preview for, falls back to the glyph.
const broken = ref(new Set<string>());
watch(() => props.st.at, () => { broken.value = new Set(); });
</script>

<template>
  <div class="cf-row" :data-i="st.at">
    <div class="cf-name"><b>{{ c.key }}</b><Badge tone="fail" size="small">{{ t('transfer.taken') }}</Badge><span v-if="many" class="cf-step glim-num">{{ t('transfer.step', { at: st.at + 1, n: conflicts.length }) }}</span></div>
    <div class="cf-cards">
      <article v-for="s in sides" :key="s.key" class="cf-card" :class="`cf-card--${s.key}`">
        <div class="cf-pic">
          <img v-if="image && !broken.has(s.key)" :src="api.thumbUrl(s.side.path, 'large', Date.parse(s.side.mtime))" alt="" @error="broken = new Set([...broken, s.key])">
          <G v-else :n="glyph" />
          <span class="cf-role">{{ s.label }}</span>
        </div>
        <dl class="cf-facts">
          <div :class="{ win: s.bigger }"><dt>{{ t('transfer.size') }}</dt><dd class="glim-num">{{ size(s.side) }}<span v-if="s.bigger" class="cf-tag">{{ t('transfer.bigger') }}</span></dd></div>
          <div :class="{ win: s.newer }"><dt>{{ t('transfer.changed') }}</dt><dd class="glim-num">{{ fmtTime(Date.parse(s.side.mtime)) }}<span v-if="s.newer" class="cf-tag">{{ t('transfer.newer') }}</span></dd></div>
        </dl>
      </article>
    </div>
  </div>
  <Checkbox v-if="more" v-model="st.all" class="cf-all" :label="t('transfer.forAll')" />
</template>

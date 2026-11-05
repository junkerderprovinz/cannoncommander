<script setup lang="ts">
import { Card, IconButton, ListRow, Pill } from '@glimstone/vue';
import { onMounted, ref } from 'vue';
import { admin, type Share } from '@/api/admin';
import { DAY, now } from '@/core/format';
import { G, glyphComponent } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { win } from '@/core/windows';
import { useUser } from '@/stores/user';
import { ago, c } from '@/features/config/shared';
import { copyText } from '@/ui/kit';
import ShareWin from '../wins/ShareWin.vue';

const shares = ref<Share[]>([]);
const load = async () => { shares.value = await admin.shares(useUser().admin()); };
onMounted(() => { load().catch(() => {}); });

const name = (s: Share) => s.name || s.paths[0]?.split('/').pop() || s.slug;
const glyph = (s: Share) => (s.kind === 'collection' ? 'IconCollector' : s.kind === 'file' ? 'IconFile' : 'IconFolder');
const left = (s: Share) => (s.expires == null ? null : s.expires - now());

// A revoked link is only taken off the list at first, so Rückgängig brings it back as it was. The server forgets it
// once the note is gone.
function revoke(s: Share) {
  shares.value = shares.value.filter((x) => x !== s);
  const later = setTimeout(() => { admin.deleteShare(s.id).catch(() => load()); }, 4000);
  toast(c('shares.revoked', { name: name(s) }), 'ok', { label: c('undo'), run: () => { clearTimeout(later); load(); } });
}
const edit = (s: Share) => win({ title: c('shares.share'), body: ShareWin, props: { share: s, saved: load } });
</script>

<template>
  <Card :title="c('shares.title')">
    <div v-if="shares.length" class="a-list">
      <ListRow v-for="(s, i) in shares" :key="s.id" :name="name(s)" :tip="s.paths.join(', ')" :data-open="i" @click="edit(s)">
        <template #tile><G :n="glyph(s)" /></template>
        <template #badges>
          <Pill v-if="s.state === 'exhausted'" tone="state">{{ c('shares.exhausted') }}</Pill>
          <Pill v-else-if="s.state === 'expired' || (left(s) ?? 0) < 0" tone="state">{{ c('shares.expired') }}</Pill>
          <Pill v-else-if="left(s) == null">{{ c('shares.noExpiry') }}</Pill>
          <Pill v-else :tone="left(s)! < 3 * DAY ? 'warn' : 'neutral'">{{ ago(s.expires!) }}</Pill>
          <Pill v-if="s.password">{{ c('shares.password') }}</Pill>
          <Pill v-if="s.limit" class="glim-num">{{ c('shares.limit', { n: s.limit }) }}</Pill>
          <Pill class="glim-num">{{ s.views === 1 ? c('shares.oneView') : c('shares.views', { n: s.views }) }}</Pill>
        </template>
        <template #actions>
          <IconButton :glyph="glyphComponent('IconCopy')" :tip="c('shares.copyLink')" @click="copyText(s.url)" />
          <IconButton :glyph="glyphComponent('IconPencil')" :tip="c('edit')" :data-open="i" @click="edit(s)" />
          <IconButton :glyph="glyphComponent('IconUnlink')" :tip="c('shares.revoke')" :data-revoke="i" @click="revoke(s)" />
        </template>
      </ListRow>
    </div>
    <div v-else class="empty"><G n="IconLink" /><div>{{ c('shares.none') }}</div></div>
  </Card>
</template>

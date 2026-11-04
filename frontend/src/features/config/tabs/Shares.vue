<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { admin, type Share } from '@/api/admin';
import { DAY, now } from '@/core/format';
import { G } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { win } from '@/core/windows';
import { useUser } from '@/stores/user';
import ARow from '../kit/ARow.vue';
import BareBtn from '../kit/BareBtn.vue';
import Card from '../kit/Card.vue';
import { ago, c, copy } from '../kit/kit';
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
      <ARow v-for="s in shares" :key="s.id" :name="name(s)" :tip="s.paths.join(', ')" data-open @click="edit(s)">
        <template #tile><G :n="glyph(s)" /></template>
        <template #badges>
          <span v-if="s.state === 'exhausted'" class="pill state">{{ c('shares.exhausted') }}</span>
          <span v-else-if="s.state === 'expired' || (left(s) ?? 0) < 0" class="pill state">{{ c('shares.expired') }}</span>
          <span v-else-if="left(s) == null" class="pill">{{ c('shares.noExpiry') }}</span>
          <span v-else class="pill" :class="{ warn: left(s)! < 3 * DAY }">{{ ago(s.expires!) }}</span>
          <span v-if="s.password" class="pill">{{ c('shares.password') }}</span>
          <span v-if="s.limit" class="pill glim-num">{{ c('shares.limit', { n: s.limit }) }}</span>
          <span class="pill glim-num">{{ s.views === 1 ? c('shares.oneView') : c('shares.views', { n: s.views }) }}</span>
        </template>
        <template #acts>
          <BareBtn g="IconCopy" :tip="c('shares.copyLink')" @click="copy(s.url)" />
          <BareBtn g="IconPencil" :tip="c('edit')" @click="edit(s)" />
          <BareBtn g="IconUnlink" :tip="c('shares.revoke')" @click="revoke(s)" />
        </template>
      </ARow>
    </div>
    <div v-else class="empty"><G n="IconLink" /><div>{{ c('shares.none') }}</div></div>
  </Card>
</template>

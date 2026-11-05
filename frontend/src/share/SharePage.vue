<script setup lang="ts">
// What someone sees who got a share link: the name and the note of the owner, the files with a preview, downloads
// one by one or as a zip, the password when the link has one, uploads when it takes them, and a kind word when the
// link leads nowhere any more.
import { Button, Card, EmptyState, FieldRow, IconButton, PasswordField, SubCard } from '@glimstone/vue';
import { computed, onMounted, ref } from 'vue';
import { ApiError } from '@/api/client';
import { inside, type PublicItem, type PublicListing, type PublicShare, shareApi } from '@/api/share';
import { fmtCount, fmtDate, fmtListDate, fmtSize } from '@/core/format';
import { G, glyphOf, glyphComponent } from '@/core/glyphs';
import { t } from '@/i18n';
import { eyeWords } from '@/ui/kit';
import { canPreview } from './preview';
import SharePreview from './SharePreview.vue';
import ShareUpload from './ShareUpload.vue';

const s = (key: string, params?: Record<string, string | number>) => t('share.' + key, params);
const slug = decodeURIComponent(location.pathname.replace(/\/+$/, '').split('/').pop() || '');
const api = shareApi(slug);

type Gone = 'expired' | 'usedUp' | 'withdrawn' | 'missing' | 'busy' | 'broken';
const state = ref<'loading' | 'locked' | 'open' | Gone>('loading');
const pub = ref<PublicShare | null>(null);
const listing = ref<PublicListing | null>(null);
const path = ref('/');
const password = ref(''), err = ref(''), busy = ref(false);
const shown = ref<PublicItem | null>(null);

const GONE: Record<string, Gone> = { 'share-expired': 'expired', 'share-used-up': 'usedUp', 'share-withdrawn': 'withdrawn' };
const GONE_GLYPH: Record<Gone, string> = { expired: 'IconHourglass', usedUp: 'IconEmpty', withdrawn: 'IconUnlink', missing: 'IconUnlink', busy: 'IconHourglass', broken: 'IconWarning' };

function whyGone(e: unknown): Gone {
  if (!(e instanceof ApiError)) return 'broken';
  if (e.status === 410) return GONE[e.problem?.type ?? ''] ?? 'withdrawn';
  if (e.status === 404 || e.status === 400) return 'missing';
  if (e.status === 429) return 'busy';
  return 'broken';
}

async function load() {
  try {
    pub.value = await api.open();
  } catch (e) {
    state.value = whyGone(e);
    return;
  }
  document.title = pub.value.name ? `${pub.value.name} · CannonCommander` : 'CannonCommander';
  if (pub.value.locked) {
    state.value = 'locked';
    return;
  }
  await browse(path.value);
  state.value = 'open';
}

async function browse(to: string) {
  try {
    listing.value = await api.list(to);
    path.value = listing.value.path;
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) state.value = 'locked';
    else if (e instanceof ApiError && e.status === 404 && to !== '/') await browse('/');
    else state.value = whyGone(e);
  }
}

onMounted(load);

async function unlock() {
  if (!password.value) {
    err.value = s('needPassword');
    return;
  }
  busy.value = true;
  try {
    await api.unlock(password.value);
    err.value = '';
    await load();
  } catch (e) {
    if (e instanceof ApiError && e.status === 403) err.value = s('wrongPassword');
    else if (e instanceof ApiError && e.status === 429) err.value = s('tooMany', { n: Math.max(1, Math.ceil((e.retryAfter ?? 300) / 60)) });
    else state.value = whyGone(e);
  } finally {
    busy.value = false;
  }
}

const isFile = computed(() => pub.value?.kind === 'file');
const items = computed(() => listing.value?.items ?? []);
const file = computed(() => (isFile.value ? items.value[0] ?? null : null));
const crumbs = computed(() => {
  const parts = path.value.split('/').filter(Boolean);
  return parts.map((name, i) => ({ name, path: '/' + parts.slice(0, i + 1).join('/') }));
});
const meta = computed(() => {
  const p = pub.value;
  if (!p) return [];
  const out: string[] = [];
  if (p.kind === 'file' && p.size != null) out.push(fmtSize(p.size));
  if (p.kind === 'file' && p.mtime) out.push(fmtDate(Date.parse(p.mtime)));
  if (p.kind === 'collection' && p.count) out.push(p.count === 1 ? s('oneEntry') : s('entries', { n: fmtCount(p.count) }));
  if (p.expires) out.push(s('until', { date: fmtDate(Date.parse(p.expires)) }));
  return out;
});
const zipLabel = computed(() => (path.value === '/' ? s('downloadAll') : s('downloadFolder')));
const filePath = (item: PublicItem) => (isFile.value ? '/' : inside(path.value, item.name));
const previewable = computed(() => items.value.filter((k) => !k.dir && canPreview(k.name)));

function openItem(item: PublicItem) {
  if (item.dir) browse(inside(path.value, item.name));
  else shown.value = item;
}
function step(by: number) {
  const list = previewable.value, i = list.findIndex((k) => k.name === shown.value?.name);
  if (list.length) shown.value = list[(i + by + list.length) % list.length]!;
}
async function uploaded() {
  try { pub.value = await api.open(); } catch { /* the list below still shows the new files */ }
  await browse(path.value);
}
</script>

<template>
  <div class="sp">
    <header class="sp-head">
      <span class="brand-mark"><G n="IconFolderOpen" /></span>
      <span class="sp-brand"><span class="brand-name">CannonCommander</span><span class="muted">{{ s('sharedWith') }}</span></span>
    </header>

    <main class="sp-main">
      <div v-if="state === 'loading'" class="sp-wait"><span class="sp-spin"></span><span class="muted">{{ s('loading') }}</span></div>

      <Card v-else-if="state === 'locked'" class="glim-card sp-card sp-narrow" :title="s('lockedTitle')">
        <form class="sp-lock" novalidate @submit.prevent="unlock">
          <p class="sp-text">{{ s('lockedText') }}</p>
          <div v-if="err" class="sp-err" role="alert"><G n="IconWarning" /><span>{{ err }}</span></div>
          <FieldRow :label="s('password')"><PasswordField id="sp-pass" v-model="password" autocomplete="current-password" width="grow" :text="eyeWords()" /></FieldRow>
          <Button class="glim-btn-key" :label="s('open')" label-key="IconUnlock" tone="accent" type="submit" :disabled="busy" />
        </form>
      </Card>

      <template v-else-if="state === 'open' && pub">
        <Card class="glim-card sp-card" :title="s('kind.' + pub.kind)">
          <div class="sp-title">
            <G :n="isFile && file ? glyphOf(file) : 'IconFolderOpen'" />
            <div class="sp-name"><h1>{{ pub.name }}</h1><div v-if="meta.length" class="muted glim-num">{{ meta.join(' · ') }}</div></div>
            <Button v-if="isFile && file && canPreview(file.name)" class="glim-btn-key" :label="s('preview')" label-key="IconEye" data-sp="preview" @click="shown = file" />
            <Button v-if="isFile" class="glim-btn-key" :label="s('download')" label-key="IconDownload" tone="accent" :href="api.rawUrl('/', true)" data-sp="download" />
            <Button v-else class="glim-btn-key" :label="zipLabel" label-key="IconDownload" tone="accent" :href="api.zipUrl(path)" data-sp="zip" />
          </div>
          <SubCard :title="s('message')" v-if="pub.message" class="sp-msg"><p>{{ pub.message }}</p></SubCard>
        </Card>

        <Card v-if="!isFile" class="glim-card sp-card sp-files" :title="s('files')">
          <nav class="sp-crumbs" :aria-label="s('path')">
            <button type="button" class="sp-crumb" :class="{ cur: path === '/' }" @click="browse('/')">{{ s('root') }}</button>
            <template v-for="c in crumbs" :key="c.path"><span class="muted">/</span><button type="button" class="sp-crumb" :class="{ cur: c.path === path }" @click="browse(c.path)">{{ c.name }}</button></template>
          </nav>
          <EmptyState v-if="!items.length" nested :glyph="glyphComponent('IconEmpty')" :title="s('empty')" />
          <div v-else class="sp-list" role="list">
            <div class="sp-lhead muted"><span>{{ s('name') }}</span><span>{{ s('size') }}</span><span>{{ s('changed') }}</span><span></span></div>
            <div v-for="k in items" :key="k.name" class="sp-row" role="listitem">
              <button type="button" class="sp-open" :data-name="k.name" @click="openItem(k)"><G :n="glyphOf(k)" /><span class="sp-nm">{{ k.name }}</span></button>
              <span class="muted glim-num">{{ k.dir ? '' : fmtSize(k.size) }}</span>
              <span class="muted glim-num">{{ fmtListDate(Date.parse(k.mtime)) }}</span>
              <span class="sp-acts">
                <IconButton v-if="!k.dir && canPreview(k.name)" :tip="s('preview')" :glyph="glyphComponent('IconEye')" @click="shown = k" />
                <IconButton :href="k.dir ? api.zipUrl(inside(path, k.name)) : api.rawUrl(inside(path, k.name), true)" :tip="s('download')" :glyph="glyphComponent('IconDownload')" />
              </span>
            </div>
          </div>
        </Card>

        <ShareUpload v-if="pub.uploads" :api="api" :dir="path" :room="pub.uploadRoom" @done="uploaded" />
      </template>

      <Card v-else class="glim-card sp-card sp-narrow sp-gone" :title="s(state + 'Title')">
        <G :n="GONE_GLYPH[state as Gone]" />
        <h1>{{ s(state + 'Title') }}</h1>
        <p class="sp-text">{{ s(state + 'Text') }}</p>
      </Card>
    </main>

    <SharePreview v-if="shown" :item="shown" :src="api.rawUrl(filePath(shown))" :download="api.rawUrl(filePath(shown), true)" :text="() => api.text(filePath(shown!))"
      :many="previewable.length > 1" @close="shown = null" @step="step" />
  </div>
</template>

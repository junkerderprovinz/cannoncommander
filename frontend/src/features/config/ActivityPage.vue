<script setup lang="ts">
// Who did what and when, filtered by person or kind from the column heads. A row naming a place opens it.
import { computed, onMounted, reactive, ref } from 'vue';
import { type Activity, admin, type User } from '@/api/admin';
import { DAY, now } from '@/core/format';
import { G } from '@/core/glyphs';
import { menu } from '@/core/menu';
import { go } from '@/core/views';
import { usePanels } from '@/stores/panels';
import Card from './kit/Card.vue';
import { ago, c } from './kit/kit';

// The server's event types and their glyphs. A type it adds later shows with its own name and the log glyph.
const KINDS: Record<string, string> = {
  copy: 'IconCopy', move: 'IconMove', rename: 'IconPencil', upload: 'IconUpload', download: 'IconDownload', fileModify: 'IconPencil',
  delete: 'IconTrash', bulkDelete: 'IconTrash', archive: 'IconArchive', unarchive: 'IconArchive', duplicateFinder: 'IconCopy',
  shareCreate: 'IconLink', shareUpdate: 'IconLink', shareDelete: 'IconUnlink',
  userCreate: 'IconAccounts', userUpdate: 'IconAccounts', userDelete: 'IconAccounts',
  accessCreate: 'IconShieldOn', accessUpdate: 'IconShieldOn', accessDelete: 'IconShieldOn',
  quotaCreate: 'IconSliders', quotaUpdate: 'IconSliders', quotaDelete: 'IconSliders',
  login: 'IconSignIn', logout: 'IconSignOut', signup: 'IconSignIn',
  passkeyRegister: 'IconKey', passkeyDelete: 'IconKey', tokenCreate: 'IconKey', tokenDelete: 'IconKey',
};
const all = ref<Activity[]>([]), users = ref<User[]>([]);
const act = reactive({ who: null as string | null, what: null as string | null, dir: -1 });
onMounted(() => {
  admin.activity().then((a) => { all.value = a; }, () => {});
  admin.users().then((u) => { users.value = u; }, () => {});
});

const full = (who: string) => users.value.find((u) => u.name === who)?.full || who;
const kind = (what: string) => (what in KINDS ? c(`activity.kinds.${what}`) : what);
const shown = computed(() => all.value.filter((a) => (!act.who || a.who === act.who) && (!act.what || a.what === act.what)).sort((a, b) => (a.t - b.t) * act.dir));
const filtered = computed(() => !!(act.who || act.what));
const today = computed(() => all.value.filter((a) => a.t >= now() - (now() % DAY)).length);
const people = computed(() => new Set(all.value.map((a) => a.who)).size);

function filter(e: MouseEvent, k: 'who' | 'what') {
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
  const values = [...new Set(all.value.map((a) => a[k]))];
  menu(r.left, r.bottom + 6, [
    { label: c('activity.all'), checked: () => !act[k], run: () => { act[k] = null; } }, '-',
    ...values.map((v) => ({ label: k === 'who' ? full(v) : kind(v), g: k === 'what' ? KINDS[v] || 'IconLog' : undefined, checked: () => act[k] === v, run: () => { act[k] = v; } })),
  ]);
}
// The tip carries what the row has no room for: where a copy went and where a login came from.
function tip(a: Activity) {
  const lines = [a.path ? c('activity.show') : '', a.target ? c('activity.target', { path: a.target }) : '', a.ip ? c('activity.from', { ip: a.ip }) : ''];
  return lines.filter(Boolean).join(' ') || undefined;
}
function open(a: Activity) {
  if (!a.path) return;
  go('files');
  usePanels().go(a.path);
}
</script>

<template>
  <div class="page-inner">
    <Card :title="c('activity.overview')">
      <div class="a-stats">
        <div class="well a-stat"><span class="glim-num">{{ filtered ? c('activity.ofAll', { n: shown.length, all: all.length }) : all.length }}</span><span>{{ filtered ? c('activity.inFilter') : c('activity.entries') }}</span></div>
        <div class="well a-stat"><span class="glim-num">{{ today }}</span><span>{{ c('activity.today') }}</span></div>
        <div class="well a-stat"><span class="glim-num">{{ people }}</span><span>{{ c('activity.people') }}</span></div>
      </div>
      <div v-if="filtered" class="a-foot"><button class="btn" @click="act.who = act.what = null"><G n="IconClose" />{{ c('activity.reset') }}</button></div>
    </Card>
    <Card :title="filtered ? c('activity.filtered') : c('activity.allEntries')">
      <div class="a-actlist">
        <div class="a-lhead"><span></span><span>{{ c('activity.what') }}</span>
          <button class="a-hbtn" :class="{ on: act.who }" data-af="who" :data-tip="c('activity.filter')" @click.stop="filter($event, 'who')">{{ act.who ? full(act.who) : c('activity.who') }}<G n="IconExpand" /></button>
          <button class="a-hbtn" :class="{ on: act.what }" data-af="what" :data-tip="c('activity.filter')" @click.stop="filter($event, 'what')">{{ act.what ? kind(act.what) : c('activity.kind') }}<G n="IconExpand" /></button>
          <button class="a-hbtn" @click="act.dir *= -1">{{ c('activity.when') }}<G :n="act.dir < 0 ? 'IconMoveDown' : 'IconMoveUp'" /></button>
        </div>
        <template v-if="shown.length">
          <div v-for="a in shown" :key="a.id" class="a-row a-arow" :tabindex="a.path ? 0 : undefined" :data-open="a.path" :data-tip="tip(a)" @click="open(a)" @keydown.enter="open(a)">
            <span class="tile"><G :n="KINDS[a.what] || 'IconLog'" /></span><span class="a-name"><span class="t">{{ a.obj }}</span></span>
            <span class="pill">{{ full(a.who) }}</span><span class="pill">{{ kind(a.what) }}</span><span class="pill glim-num">{{ ago(a.t) }}</span>
          </div>
        </template>
        <div v-else class="empty"><G n="IconFilter" /><div>{{ c('activity.nothing') }}</div></div>
      </div>
    </Card>
  </div>
</template>

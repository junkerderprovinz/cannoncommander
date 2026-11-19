<script setup lang="ts">
import { segItems } from '@/ui/kit';
import { Button, Card, ConnectionRow, EmptyState, FieldRow, Pill, Selector, Toggle, type ConnectionTest } from '@glimstone/vue';
import { computed, onMounted, reactive, ref, shallowRef } from 'vue';
import { admin, type AppSettings, type Provider, type Rclone, type Remote, type RemoteTest } from '@/api/admin';
import { glyphComponent } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { attempt, c, confirmWin } from '@/features/config/shared';
import { reloadRemotes } from '@/features/remotes/store';
import { loadCatalog, markOf, remoteKind, testVerdict } from '../remotes';
import RemoteWin from '../wins/RemoteWin.vue';

type Mounting = AppSettings['remotes'];
const remotes = ref<Remote[]>([]);
const mounting = ref<Mounting | null>(null);
// The container brings rclone along. On a computer of one's own it has to be installed first.
const rclone = ref<Rclone | null>(null);
// The products give each row its name and logo. Until rclone answers, a row says its rclone type.
const providers = ref<Provider[]>([]);
const load = async () => { ({ remotes: remotes.value, rclone: rclone.value } = await admin.remotes()); };
const markFor = (r: Remote) => {
  const m = markOf(providers.value.some((p) => p.id === r.provider) ? r.provider! : '');
  return m.mark ?? glyphComponent(m.glyph);
};
onMounted(() => {
  load().then(() => { if (rclone.value?.running) loadCatalog().then((c) => { providers.value = c.providers; }, () => {}); }, () => {});
  admin.app().then((a) => { mounting.value = a.remotes; }, () => {});
});

function saveRc(change: Partial<Mounting>) {
  if (!mounting.value) return;
  mounting.value = { ...mounting.value, ...change };
  admin.saveApp({ remotes: change }).then(() => toast(c('saved')), () => toast(c('notSaved'), 'warn'));
}
const automount = computed({ get: () => mounting.value?.automount ?? true, set: (v: boolean) => saveRc({ automount: v }) });
const cache = computed({ get: () => mounting.value?.cache ?? 'writes', set: (v: Mounting['cache']) => saveRc({ cache: v }) });
const cacheSize = computed({ get: () => mounting.value?.cacheSize ?? 50, set: (v: number) => saveRc({ cacheSize: v }) });

async function mount(r: Remote) {
  if (!(await attempt(admin.mountRemote(r.name, !r.mounted)))) return load();
  await load();
  void reloadRemotes();
  toast(r.mounted ? c('remotes.unmounted', { name: r.name }) : c('remotes.mounted', { name: r.name }));
}
// The last test of each remote, by name, which the row's Test button wears.
const tests = reactive<Record<string, ConnectionTest>>({});
async function test(r: Remote) {
  tests[r.name] = { busy: true };
  const got = await admin.testRemote({ name: r.name, type: r.type, path: r.path, mount: r.mount, params: { ...(r.params ?? {}) } })
    .catch((e: unknown): RemoteTest => ({ ok: false, entries: 0, error: e instanceof Error ? e.message : String(e) }));
  // A question from rclone's setup is answered in the remote's own window.
  tests[r.name] = got.question ? { tone: 'danger', title: c('remotes.notConnected') } : { tone: got.ok ? 'ok' : 'danger', title: testVerdict(got, r.path) };
}
const rowText = computed(() => ({ test: c('test'), edit: c('edit'), remove: c('remove') }));
const ROW_KEYS = { test: 'IconHealth', edit: 'IconPencil', remove: 'IconTrash' };

// The remote in the window, null for a new one.
const editing = shallowRef<{ remote: Remote | null } | null>(null);
const edit = (r: Remote | null) => { editing.value = { remote: r }; };
function saved() {
  void load();
  void reloadRemotes();
}
function remove(r: Remote) {
  confirmWin({
    title: c('remotes.remove'), verb: c('remove'), glyph: 'IconTrash',
    text: [c('remotes.removeText1', { name: r.name }), c('remotes.removeText2', { type: remoteKind(r, providers.value) })],
    run: async () => {
      if (!(await attempt(admin.deleteRemote(r.name)))) return;
      await load();
      void reloadRemotes();
      toast(c('removed', { name: r.name }));
    },
  });
}
</script>

<template>
  <Card :title="c('remotes.title')">
    <!-- Mounting stands beside the row until GlimStone's ConnectionRow has a place for an action of its own. -->
    <div v-if="remotes.length" class="a-conns">
      <div v-for="r in remotes" :key="r.name" class="a-conn">
        <ConnectionRow :name="r.name" :mark="markFor(r)" :status="remoteKind(r, providers)" :test="tests[r.name]" :text="rowText" :label-keys="ROW_KEYS"
          @test="test(r)" @edit="edit(r)" @remove="remove(r)">
          <template #badges>
            <Pill v-if="r.error" tone="warn" :title="c('remotes.mountFailed', { error: r.error })">{{ c('remotes.failedPill') }}</Pill>
            <Pill v-else-if="!r.mounted" tone="state">{{ c('remotes.unmountedPill') }}</Pill>
          </template>
          <p class="a-conn-path mono">{{ r.mount }}</p>
        </ConnectionRow>
        <Button data-mount :label="r.mounted ? c('remotes.unmount') : c('remotes.mount')" :label-key="r.mounted ? 'IconUnlink' : 'IconLink'" :compact-below="640" @click="mount(r)" />
      </div>
    </div>
    <EmptyState v-else-if="rclone && !rclone.installed" nested :glyph="glyphComponent('IconCloud')" :title="c('remotes.noRclone')" :hint="c('remotes.noRcloneHint')" />
    <EmptyState v-else nested :glyph="glyphComponent('IconCloud')" :title="c('remotes.none')" />
    <div class="a-foot"><Button data-add :label="c('remotes.add')" label-key="IconAdd" @click="edit(null)" /></div>
  </Card>
  <Card :title="c('remotes.mounting')">
    <FieldRow :label="c('remotes.automount')" :hint="c('remotes.automountTip')"><Toggle v-model="automount" /></FieldRow>
    <FieldRow :label="c('remotes.cache')" :hint="c('remotes.cacheTip')"><Selector v-model="cache" :items="segItems([['off', c('remotes.cacheOff')], ['writes', c('remotes.cacheWrites')], ['full', c('remotes.cacheFull')]])" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    <FieldRow v-if="cache !== 'off'" :label="c('remotes.atMost')" :hint="c('remotes.atMostTip')"><Selector v-model="cacheSize" :items="segItems([[10, '10 GB'], [50, '50 GB'], [200, '200 GB']])" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
  </Card>
  <RemoteWin v-if="editing" :remote="editing.remote" :all="remotes" @saved="saved" @close="editing = null" />
</template>

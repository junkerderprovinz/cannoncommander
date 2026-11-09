<script setup lang="ts">
// Where an MCP key comes from, and the only place it is ever shown: the server keeps a fingerprint, so a key nobody
// copied has to be replaced. Every account manages its own keys here. Admins also see everybody else's, may take
// writing away or revoke them, and switch the sign-in through OAuth.
import { Button, Card, CopyField, EmptyState, FieldRow, IconButton, ListRow, Pill, SubCard, Toggle } from '@glimstone/vue';
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { type Account, me } from '@/api/admin';
import { mcp, type McpKey, type McpKeys, type McpOAuth } from '@/api/mcp';
import { on } from '@/api/sse';
import { fmtDate } from '@/core/format';
import { glyphComponent } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { win } from '@/core/windows';
import { ago, attempt, c, cn, confirmWin } from '@/features/config/shared';
import { useUser } from '@/stores/user';
import { copyWords } from '@/ui/kit';
import { takePending } from '../proof';
import { type Step, takeStep } from './again';
import ClientMark from './ClientMark.vue';
import ClientWin from './ClientWin.vue';
import { clientById, type McpClient } from './clients';
import LogWin from './LogWin.vue';
import NameWin from './NameWin.vue';
import OAuthRows from './OAuthRows.vue';
import PickWin from './PickWin.vue';
import ProofWin from './ProofWin.vue';
import { mcpUrl } from './snippets';

const user = useUser();
const data = ref<McpKeys | null>(null), every = ref<McpKeys | null>(null), acc = ref<Account | null>(null);
const showRevoked = ref(false);
const admin = computed(() => user.admin());

async function load() {
  const [mine, all] = await Promise.allSettled([mcp.keys(), admin.value ? mcp.keys(true) : Promise.reject(new Error('not an admin'))]);
  if (mine.status === 'fulfilled') data.value = mine.value;
  if (all.status === 'fulfilled') every.value = all.value;
}

const keys = computed(() => data.value?.keys ?? []);
const revoked = computed(() => data.value?.revoked ?? []);
const others = computed(() => every.value?.keys.filter((k) => !k.own) ?? []);
const working = computed(() => keys.value.filter((k) => !k.unusable).length);
const status = computed(() => {
  if (working.value) return cn(working.value, 'mcp.statusOn');
  if (data.value?.oauth.active) return c('mcp.statusOAuth');
  return keys.value.length ? c('mcp.statusNone') : c('mcp.statusOff');
});
// New keys, replacements and writing ask for the password of an account that has one; one that signs in elsewhere
// signs in afresh instead, and a server nobody signs in to asks for nothing.
const password = computed(() => !!data.value?.signIn && acc.value?.loginMethod !== 'oidc');

const host = location.hostname;
const base = () => new URL(import.meta.env.BASE_URL, location.href).pathname;
const endpointParts = computed(() => ({ origin: location.origin + base(), endpointPath: data.value?.endpointPath ?? 'mcp' }));
const endpoint = computed(() => mcpUrl(endpointParts.value));
const connector = computed(() => (data.value?.oauth.issuer ? mcpUrl({ origin: data.value.oauth.issuer + base(), endpointPath: data.value.endpointPath }) : ''));
const limits = computed(() => ({ perCall: data.value?.writeLimits.entriesPerCall ?? 0, perHour: data.value?.writeLimits.entriesPerHour ?? 0 }));

function facts(k: McpKey) {
  const client = clientById(k.client);
  const made = k.rotatedAt ? c('mcp.rotatedAt', { date: fmtDate(k.rotatedAt) }) : c('mcp.createdAt', { date: fmtDate(k.createdAt) });
  return [client?.name, made, k.unusable ? c('mcp.unusableTip') : ''].filter(Boolean).join(' ');
}

// Switching sign-in through OAuth off, or moving its address, ends every grant, so the keys are read again.
function oauthChanged(oauth: McpOAuth) {
  if (data.value) data.value.oauth = oauth;
  void load();
}

function connect(client: McpClient) {
  win({
    title: c('mcp.dialogTitle', { name: client.name || c('mcp.other') }), wide: true, body: ClientWin,
    props: {
      client, state: () => data.value, reload: load, password: password.value, admin: admin.value, endpoint: endpointParts.value, connector: connector.value,
      made: (got: { item: McpKey }) => { toast(c('mcp.made', { name: got.item.label })); void load(); }, oauthChanged,
    },
  });
}
const choose = () => win({ title: c('mcp.choose'), wide: true, body: PickWin, props: { cloud: !data.value?.desktop, pick: connect } });

function proof(title: string, text: string[], verb: string, glyph: string, step: Step, run: (password?: string) => Promise<string | void>, done: () => void) {
  win({ title, body: ProofWin, props: { text, verb, glyph, password: password.value, step, run, done } });
}
function setWrite(k: McpKey, allow: boolean) {
  if (!allow) {
    void attempt(mcp.update(k.id, { canWrite: false })).then((ok) => { if (ok) toast(c('mcp.writeOff', { name: k.label })); return load(); });
    return;
  }
  proof(c('mcp.writeTitle'), [c('mcp.writeAsk', { name: k.label }), c('mcp.writeTip', limits.value)], c('mcp.consent.allow'), 'IconUnlock', { do: 'write', id: k.id },
    async (pw) => { await mcp.update(k.id, { canWrite: true, password: pw }); },
    () => { toast(c('mcp.writeOn', { name: k.label })); void load(); });
}
function replace(k: McpKey) {
  proof(c('mcp.replaceTitle'), [c('mcp.replaceText', { name: k.label })], c('mcp.replace'), 'IconRetry', { do: 'rotate', id: k.id },
    async (pw) => (await mcp.rotate(k.id, pw)).key,
    () => { toast(c('mcp.replaced', { name: k.label })); void load(); });
}
const rename = (k: McpKey) => win({
  title: c('mcp.rename'), body: NameWin,
  props: { name: k.label, save: async (label: string) => { await mcp.update(k.id, { label }); toast(c('mcp.renamed')); await load(); } },
});
const log = (k: McpKey) => win({ title: k.label, body: LogWin, props: { k } });
function revoke(k: McpKey) {
  confirmWin({ title: c('mcp.revokeTitle'), verb: c('security.revoke'), glyph: 'IconKeyRevoke', text: [c('mcp.revokeText', { name: k.label })],
    run: async () => { if (!(await attempt(mcp.revoke(k.id)))) return; toast(c('mcp.revoked', { name: k.label })); await load(); } });
}
function purge(k: McpKey) {
  confirmWin({ title: c('mcp.purgeTitle'), verb: c('remove'), glyph: 'IconTrash', text: [c('mcp.purgeText', { name: k.label })],
    run: async () => { if (!(await attempt(mcp.purge(k.id)))) return; toast(c('removed', { name: k.label })); await load(); } });
}

// Back from a fresh sign-in, the step that asked for it is taken up again.
function resume(step: Step | null) {
  if (!step) return;
  if (step.do === 'create') {
    const client = clientById(step.client);
    if (client) connect(client);
    return;
  }
  const k = keys.value.find((x) => x.id === step.id);
  if (k && step.do === 'write') setWrite(k, true);
  else if (k) replace(k);
}

let stop = () => {};
onMounted(async () => {
  stop = on('mcp.changed', () => { void load(); });
  const [, a] = await Promise.allSettled([load(), me.account()]);
  if (a.status === 'fulfilled') acc.value = a.value;
  if (takePending() === 'mcp') resume(takeStep());
});
onBeforeUnmount(() => stop());
// The account is known a moment after the page starts, and only then does an admin's list come.
watch(admin, (yes) => { if (yes) void load(); });
</script>

<template>
  <Card :title="c('mcp.title')" :hint="c('mcp.hint')" data-mcp="card">
    <FieldRow :label="c('mcp.status')"><Pill :tone="working ? 'ok' : 'neutral'" data-mcp="status">{{ status }}</Pill></FieldRow>
    <template v-if="data">
      <FieldRow :label="c('mcp.endpoint')" :hint="c('mcp.endpointTip')"><CopyField width="grow" :value="endpoint" :label="c('mcp.endpoint')" :text="copyWords()" data-mcp="endpoint" /></FieldRow>
      <SubCard v-if="data.desktop" class="a-text" data-mcp="port"><p>{{ c('mcp.desktopPort') }}</p></SubCard>
      <SubCard v-if="!data.hostAllowsKeys" class="a-text"><p>{{ c('mcp.hostRefused', { host }) }}</p></SubCard>
      <FieldRow v-if="data.oauth.active && connector" :label="c('mcp.connector')" :hint="c('mcp.connectorTip')"><CopyField width="grow" :value="connector" :label="c('mcp.connector')" :text="copyWords()" /></FieldRow>
      <FieldRow :label="c('mcp.connect')" :hint="c('mcp.connectTip')"><Button id="mcp-choose" :label="c('mcp.choose')" label-key="IconLink" @click="choose" /></FieldRow>
    </template>
    <SubCard :title="c('mcp.keys')">
      <div v-if="keys.length" class="a-list" data-mcp="keys">
        <ListRow v-for="k in keys" :key="k.id" :name="k.label" :tip="facts(k)" :data-key="k.id">
          <template #tile><span class="mcp-row-mark"><ClientMark :client="clientById(k.client)" for-key /></span></template>
          <template #badges>
            <Pill v-if="k.unusable" tone="warn">{{ c('mcp.unusable') }}</Pill>
            <Pill v-if="!k.viaOAuth" class="mono">…{{ k.hint }}</Pill>
            <Pill class="glim-num">{{ cn(k.callsToday, 'mcp.callsToday') }}</Pill>
            <Pill>{{ k.lastUsedAt ? c('security.used', { ago: ago(k.lastUsedAt) }) : c('security.neverUsed') }}</Pill>
          </template>
          <template #actions>
            <Toggle :model-value="k.canWrite" :label="c('mcp.write')" data-mcp="write" @update:model-value="setWrite(k, $event)" />
            <IconButton :absent="k.viaOAuth" :glyph="glyphComponent('IconPencil')" :tip="c('mcp.rename')" data-mcp="rename" @click="rename(k)" />
            <IconButton :absent="k.viaOAuth || !data?.hostAllowsKeys" :glyph="glyphComponent('IconRetry')" :tip="c('mcp.replace')" data-mcp="replace" @click="replace(k)" />
            <IconButton :glyph="glyphComponent('IconLog')" :tip="c('mcp.log')" data-mcp="log" @click="log(k)" />
            <IconButton :glyph="glyphComponent('IconKeyRevoke')" :tip="c('security.revoke')" data-mcp="revoke" @click="revoke(k)" />
          </template>
        </ListRow>
      </div>
      <EmptyState v-else nested :glyph="glyphComponent('IconKey')" :title="c('mcp.noKeys')" />
      <template v-if="revoked.length">
        <div class="a-foot"><Button :label="cn(revoked.length, 'mcp.revokedList')" :label-key="showRevoked ? 'IconCollapse' : 'IconExpand'" data-mcp="revoked" @click="showRevoked = !showRevoked" /></div>
        <div v-if="showRevoked" class="a-list">
          <ListRow v-for="k in revoked" :key="k.id" :name="k.label" :tip="c('mcp.revokedAt', { date: fmtDate(k.revokedAt) })">
            <template #tile><span class="mcp-row-mark mcp-muted"><ClientMark :client="clientById(k.client)" for-key /></span></template>
            <template #actions>
              <IconButton :glyph="glyphComponent('IconLog')" :tip="c('mcp.log')" @click="log(k)" />
              <IconButton :glyph="glyphComponent('IconTrash')" :tip="c('remove')" data-mcp="purge" @click="purge(k)" />
            </template>
          </ListRow>
        </div>
      </template>
    </SubCard>
  </Card>
  <Card v-if="admin && every && data" :title="c('mcp.all')" :hint="c('mcp.allTip')" data-mcp="admin">
    <OAuthRows v-if="!data.desktop" :oauth="data.oauth" :connector="connector" :changed="oauthChanged" />
    <div v-if="others.length" class="a-list" data-mcp="others">
      <ListRow v-for="k in others" :key="k.id" :name="k.label" :tip="facts(k)" :data-key="k.id">
        <template #tile><span class="mcp-row-mark"><ClientMark :client="clientById(k.client)" for-key /></span></template>
        <template #badges>
          <Pill tone="state">{{ k.owner }}</Pill>
          <Pill class="glim-num">{{ cn(k.callsToday, 'mcp.callsToday') }}</Pill>
          <Pill>{{ k.lastUsedAt ? c('security.used', { ago: ago(k.lastUsedAt) }) : c('security.neverUsed') }}</Pill>
        </template>
        <template #actions>
          <Toggle :model-value="k.canWrite" :label="c('mcp.write')" :refused="k.canWrite ? undefined : c('mcp.onlyOwner', { owner: k.owner })" data-mcp="write" @update:model-value="setWrite(k, $event)" />
          <IconButton :glyph="glyphComponent('IconLog')" :tip="c('mcp.log')" @click="log(k)" />
          <IconButton :glyph="glyphComponent('IconKeyRevoke')" :tip="c('security.revoke')" data-mcp="revoke" @click="revoke(k)" />
        </template>
      </ListRow>
    </div>
    <EmptyState v-else nested :glyph="glyphComponent('IconAccounts')" :title="c('mcp.noOthers')" />
  </Card>
</template>
